/*
Copyright 2021.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package podlister

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/sustainable-computing-io/kepler/pkg/config"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
)

type ContainerInfo struct {
	ContainerID   string
	ContainerName string
	PodName       string
	Namespace     string
}

const (
	systemProcessName      string = "system_processes"
	systemProcessNamespace string = "system"
	unknownPath            string = "unknown"

	procPath   string = "/proc/%d/cgroup"
	cgroupPath string = "/sys/fs/cgroup"
)

var (
	byteOrder binary.ByteOrder = determineHostByteOrder()
	podLister KubeletPodLister = KubeletPodLister{}

	// map to cache data to speedup lookups
	containerIDCache           = map[uint64]string{}
	containerIDToContainerInfo = map[string]*ContainerInfo{}
	cGroupIDToPath             = map[uint64]string{}

	// regex to extract container ID from path
	regexFindContainerIDPath          = regexp.MustCompile(`.*-(.*?)\.scope`)
	regexReplaceContainerIDPathPrefix = regexp.MustCompile(`.*-`)
	// some platforms (e.g. RHEL) have different cgroup path
	regexFindContainerIDPath2 = regexp.MustCompile(`[^:]*$`)

	regexReplaceContainerIDPathSufix = regexp.MustCompile(`\..*`)
	regexReplaceContainerIDPrefix    = regexp.MustCompile(`.*//`)
)

func Init() (*[]corev1.Pod, error) {
	return updateListPodCache("", false)
}

func GetSystemProcessName() string {
	return systemProcessName
}

func GetSystemProcessNamespace() string {
	return systemProcessNamespace
}

func GetPodName(cGroupID, pid uint64) (string, error) {
	info, err := getContainerInfo(cGroupID, pid)
	return info.PodName, err
}

func GetPodNameSpace(cGroupID, pid uint64) (string, error) {
	info, err := getContainerInfo(cGroupID, pid)
	return info.Namespace, err
}

func GetContainerName(cGroupID, pid uint64) (string, error) {
	info, err := getContainerInfo(cGroupID, pid)
	return info.ContainerName, err
}

func GetContainerID(cGroupID, pid uint64) (string, error) {
	info, err := getContainerInfo(cGroupID, pid)
	return info.ContainerID, err
}

func GetContainerMetrics() (containerCPU, containerMem map[string]float64, nodeCPU, nodeMem float64, retErr error) {
	return podLister.ListMetrics()
}

func GetAvailableKubeletMetrics() []string {
	return podLister.GetAvailableMetrics()
}

func getContainerInfo(cGroupID, pid uint64) (*ContainerInfo, error) {
	var err error
	var containerID string
	info := &ContainerInfo{
		ContainerID:   systemProcessName,
		ContainerName: systemProcessName,
		PodName:       systemProcessName,
		Namespace:     systemProcessNamespace,
	}

	if containerID, err = getContainerIDFromPath(cGroupID, pid); err != nil {
		return info, err
	}

	if i, ok := containerIDToContainerInfo[containerID]; ok {
		return i, nil
	}

	// update cache info and stop loop if container id found
	_, _ = updateListPodCache(containerID, true)
	if cinfo, ok := containerIDToContainerInfo[containerID]; ok {
		return cinfo, nil
	}

	// in the case the containerID is not a kubernetes container, the updateListPodCache will not add it to the cache
	if _, ok := containerIDToContainerInfo[containerID]; !ok {
		// some system process might have container ID, but we need to replace it if the container is not a kubernetes container
		if info.ContainerName == systemProcessName {
			containerID = systemProcessName
		}
	}

	containerIDToContainerInfo[containerID] = info
	return containerIDToContainerInfo[containerID], nil
}

// updateListPodCache updates cache info with all pods and optionally
// stops the loop when a given container ID is found
func updateListPodCache(targetContainerID string, stopWhenFound bool) (*[]corev1.Pod, error) {
	pods, err := podLister.ListPods()
	if err != nil {
		klog.V(4).Infof("%v", err)
		return pods, err
	}
	for i := 0; i < len(*pods); i++ {
		containers := (*pods)[i].Status.ContainerStatuses
		for j := 0; j < len(containers); j++ {
			containerID := ParseContainerIDFromPodStatus(containers[j].ContainerID)
			containerIDToContainerInfo[containerID] = &ContainerInfo{
				ContainerID:   containerID,
				ContainerName: containers[j].Name,
				PodName:       (*pods)[i].Name,
				Namespace:     (*pods)[i].Namespace,
			}
			if stopWhenFound && containers[j].ContainerID == targetContainerID {
				return pods, err
			}
		}
		containers = (*pods)[i].Status.InitContainerStatuses
		for j := 0; j < len(containers); j++ {
			containerID := ParseContainerIDFromPodStatus(containers[j].ContainerID)
			containerIDToContainerInfo[containerID] = &ContainerInfo{
				ContainerID:   containerID,
				ContainerName: containers[j].Name,
				PodName:       (*pods)[i].Name,
				Namespace:     (*pods)[i].Namespace,
			}
			if stopWhenFound && containers[j].ContainerID == targetContainerID {
				return pods, err
			}
		}
		containers = (*pods)[i].Status.EphemeralContainerStatuses
		for j := 0; j < len(containers); j++ {
			containerID := ParseContainerIDFromPodStatus(containers[j].ContainerID)
			containerIDToContainerInfo[containerID] = &ContainerInfo{
				ContainerID:   containerID,
				ContainerName: containers[j].Name,
				PodName:       (*pods)[i].Name,
				Namespace:     (*pods)[i].Namespace,
			}
			if stopWhenFound && containers[j].ContainerID == targetContainerID {
				return pods, err
			}
		}
	}
	return pods, err
}

func ParseContainerIDFromPodStatus(containerID string) string {
	return regexReplaceContainerIDPrefix.ReplaceAllString(containerID, "")
}

func getContainerIDFromPath(cGroupID, pid uint64) (string, error) {
	var err error
	var containerID string
	if config.EnabledEBPFCgroupID {
		containerID, err = getContainerIDFromcGroupID(cGroupID)
	} else {
		containerID, err = getContainerIDFromPID(pid)
	}
	return containerID, err
}

func getContainerIDFromPID(pid uint64) (string, error) {
	if p, ok := containerIDCache[pid]; ok {
		return p, nil
	}

	var err error
	var path string
	if path, err = getPathFromPID(procPath, pid); err != nil {
		return systemProcessName, err
	}

	containerIDCache[pid], err = extractPodContainerIDfromPath(path)
	return containerIDCache[pid], err
}

func getPathFromPID(searchPath string, pid uint64) (string, error) {
	path := fmt.Sprintf(searchPath, pid)
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("failed to open cgroup description file for pid %d: %v", pid, err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, "pod") || strings.Contains(line, "containerd") || strings.Contains(line, "crio") {
			return line, nil
		}
	}
	return "", fmt.Errorf("could not find cgroup description entry for pid %d", pid)
}

func getContainerIDFromcGroupID(cGroupID uint64) (string, error) {
	if id, ok := containerIDCache[cGroupID]; ok {
		return id, nil
	}

	var err error
	var path string
	if path, err = getPathFromcGroupID(cGroupID); err != nil {
		return systemProcessName, err
	}

	containerIDCache[cGroupID], err = extractPodContainerIDfromPath(path)
	return containerIDCache[cGroupID], err
}

// getPathFromcGroupID uses cgroupfs to get cgroup path from id
// it needs cgroup v2 (per https://github.com/iovisor/bpftrace/issues/950) and kernel 4.18+ (https://github.com/torvalds/linux/commit/bf6fa2c893c5237b48569a13fa3c673041430b6c)
func getPathFromcGroupID(cgroupID uint64) (string, error) {
	if p, ok := cGroupIDToPath[cgroupID]; ok {
		return p, nil
	}

	err := filepath.WalkDir(cgroupPath, func(path string, dentry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if !dentry.IsDir() {
			return nil
		}
		getCgroupID, err := getCgroupIDFromPath(byteOrder, path)
		if err != nil {
			return fmt.Errorf("error resolving handle: %v", err)
		}
		cGroupIDToPath[getCgroupID] = path
		return nil
	})

	if err != nil {
		return unknownPath, fmt.Errorf("failed to find cgroup id: %v", err)
	}
	if p, ok := cGroupIDToPath[cgroupID]; ok {
		return p, nil
	}

	cGroupIDToPath[cgroupID] = unknownPath
	return cGroupIDToPath[cgroupID], nil
}

// Get containerID from path. cgroup v1 and cgroup v2 will use different regex
func extractPodContainerIDfromPath(path string) (string, error) {
	cgroup := config.GetCGroupVersion()
	if regexFindContainerIDPath.MatchString(path) {
		sub := regexFindContainerIDPath.FindAllString(path, -1)
		for _, element := range sub {
			if cgroup == 2 && (strings.Contains(element, "-conmon-") || strings.Contains(element, ".service")) {
				return "", fmt.Errorf("process is not in a kubernetes pod")
				// TODO: we need to extend this to include other runtimes
			} else if strings.Contains(element, "crio") || strings.Contains(element, "docker") || strings.Contains(element, "containerd") {
				containerID := regexReplaceContainerIDPathPrefix.ReplaceAllString(element, "")
				containerID = regexReplaceContainerIDPathSufix.ReplaceAllString(containerID, "")
				return containerID, nil
			}
		}
	}
	// as some platforms (e.g. RHEL) have a different cgroup path, if the cgroup path has information from a pod and we can't get the container id
	// with the previous regex, we'll try to get it using a different approach
	if regexFindContainerIDPath2.MatchString(path) {
		sub := regexFindContainerIDPath2.FindAllString(path, -1)
		for _, containerID := range sub {
			return containerID, nil
		}
	}
	return systemProcessName, fmt.Errorf("failed to find pod's container id")
}

// GetAliveContainers returns alive pod map
func GetAliveContainers() (map[string]bool, error) {
	pods, err := podLister.ListPods()
	aliveContainers := make(map[string]bool)
	if err != nil {
		return aliveContainers, err
	}
	for i := 0; i < len(*pods); i++ {
		statuses := (*pods)[i].Status.InitContainerStatuses
		for j := 0; j < len(statuses); j++ {
			containerID := ParseContainerIDFromPodStatus(statuses[j].ContainerID)
			aliveContainers[containerID] = true
		}
		statuses = (*pods)[i].Status.ContainerStatuses
		for j := 0; j < len(statuses); j++ {
			containerID := ParseContainerIDFromPodStatus(statuses[j].ContainerID)
			aliveContainers[containerID] = true
		}
		statuses = (*pods)[i].Status.EphemeralContainerStatuses
		for j := 0; j < len(statuses); j++ {
			containerID := ParseContainerIDFromPodStatus(statuses[j].ContainerID)
			aliveContainers[containerID] = true
		}
	}
	return aliveContainers, nil
}
