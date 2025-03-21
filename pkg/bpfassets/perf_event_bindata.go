// Code generated by go-bindata.
// sources:
// bpfassets/perf_event/perf_event.c
// DO NOT EDIT!

package bpfassets

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type asset struct {
	bytes []byte
	info  os.FileInfo
}

type bindataFileInfo struct {
	name    string
	size    int64
	mode    os.FileMode
	modTime time.Time
}

func (fi bindataFileInfo) Name() string {
	return fi.name
}
func (fi bindataFileInfo) Size() int64 {
	return fi.size
}
func (fi bindataFileInfo) Mode() os.FileMode {
	return fi.mode
}
func (fi bindataFileInfo) ModTime() time.Time {
	return fi.modTime
}
func (fi bindataFileInfo) IsDir() bool {
	return false
}
func (fi bindataFileInfo) Sys() interface{} {
	return nil
}

var _bpfassetsPerf_eventPerf_eventC = []byte(`/*
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

#include <uapi/linux/ptrace.h>
#include <uapi/linux/bpf_perf_event.h>

#ifndef NUM_CPUS
#define NUM_CPUS 128
#endif

// we cannot define it dynamically as NUM_CPUS because the golang needs to know this
// size at compiler time for decoding
#define CPU_VECTOR_SIZE 128

typedef struct switch_args
{
    u64 pad;
    char prev_comm[16];
    int prev_pid;
    int prev_prio;
    long long prev_state;
    char next_comm[16];
    int next_pid;
    int next_prio;
} switch_args;

typedef struct process_time_t
{
    u64 cgroup_id;
    u64 pid;
    u64 process_run_time;
    u64 cpu_cycles;
    u64 cpu_instr;
    u64 cache_misses;
    char comm[16];
    //u64 pad;
    // the max eBPF stack limit is 512 bytes, which is a vector of u16 with 128 elements
    // the time is calculated in miliseconds, uint16 max size is 65K, ~1mim
    u16 cpu_time[CPU_VECTOR_SIZE];
}  process_time_t;

typedef struct pid_time_t
{
    int pid;
} pid_time_t;

BPF_PERF_OUTPUT(events);

// processes and pid time
BPF_HASH(processes, u64, process_time_t);
BPF_HASH(pid_time, pid_time_t);

// perf counters
BPF_PERF_ARRAY(cpu_cycles, NUM_CPUS);
BPF_PERF_ARRAY(cpu_instr, NUM_CPUS);
BPF_PERF_ARRAY(cache_miss, NUM_CPUS);

// tracking counters
BPF_ARRAY(prev_cpu_cycles, u64, NUM_CPUS);
BPF_ARRAY(prev_cpu_instr, u64, NUM_CPUS);
BPF_ARRAY(prev_cache_miss, u64, NUM_CPUS);

static void safe_array_add(u32 idx, u16 *array, u16 value)
{
#pragma clang loop unroll(full)
    for (int array_index = 0; array_index < CPU_VECTOR_SIZE-1; array_index++)
    {
        if (array_index == idx)
        {
            array[array_index] += value;
            break;
        }
    }
}

int sched_switch(switch_args *ctx)
{
    u64 pid = bpf_get_current_pid_tgid() >> 32;
#ifdef SET_GROUP_ID
    u64 cgroup_id = bpf_get_current_cgroup_id();
#else
    u64 cgroup_id = 0;
#endif

    u64 time = bpf_ktime_get_ns();
    u64 delta = 0;
    u32 cpu_id = bpf_get_smp_processor_id();
    pid_time_t new_pid, old_pid;

    // get pid time
    old_pid.pid = ctx->prev_pid;
    u64 *last_time = pid_time.lookup(&old_pid);
    if (last_time != 0)
    {
        delta = (time - *last_time) / 1000000; /*milisecond*/
        // return if the process did not use any cpu time yet
        if (delta == 0)
        {
            return 0;
        }
        pid_time.delete(&old_pid);
    }

    new_pid.pid = ctx->next_pid;
    pid_time.lookup_or_try_init(&new_pid, &time);

    u64 cpu_cycles_delta = 0;
    u64 cpu_instr_delta = 0;
    u64 cache_miss_delta = 0;
    u64 *prev;

    u64 val = cpu_cycles.perf_read(CUR_CPU_IDENTIFIER);
    if (((s64)val > 0) || ((s64)val < -256))
    {
        prev = prev_cpu_cycles.lookup(&cpu_id);
        if (prev)
        {
            cpu_cycles_delta = val - *prev;
        }
        prev_cpu_cycles.update(&cpu_id, &val);
    }
    val = cpu_instr.perf_read(CUR_CPU_IDENTIFIER);
    if (((s64)val > 0) || ((s64)val < -256))
    {
        prev = prev_cpu_instr.lookup(&cpu_id);
        if (prev)
        {
            cpu_instr_delta = val - *prev;
        }
        prev_cpu_instr.update(&cpu_id, &val);
    }
    val = cache_miss.perf_read(CUR_CPU_IDENTIFIER);
    if (((s64)val > 0) || ((s64)val < -256))
    {
        prev = prev_cache_miss.lookup(&cpu_id);
        if (prev)
        {
            cache_miss_delta = val - *prev;
        }
        prev_cache_miss.update(&cpu_id, &val);
    }

    // init process time
    struct process_time_t *process_time;
    process_time = processes.lookup(&pid);
    if (process_time == 0)
    {
        process_time_t new_process = {};
        new_process.pid = pid;
        new_process.cgroup_id = cgroup_id;
        new_process.cpu_cycles = cpu_cycles_delta;
        new_process.cpu_instr = cpu_instr_delta;
        new_process.cache_misses = cache_miss_delta;
        new_process.process_run_time += delta;
#ifdef CPU_FREQ
        //FIXME: for certain reason, hyper-v seems to always get a cpu_id that is same as NUM_CPUS and cause stack overrun
        safe_array_add(cpu_id, new_process.cpu_time, delta);
#endif        
        bpf_get_current_comm(&new_process.comm, sizeof(new_process.comm));
        processes.update(&pid, &new_process);
    }
    else
    {
        // update process time
        process_time->cpu_cycles += cpu_cycles_delta;
        process_time->cpu_instr += cpu_instr_delta;
        process_time->cache_misses += cache_miss_delta;
        process_time->process_run_time += delta;
#ifdef CPU_FREQ
        safe_array_add(cpu_id, process_time->cpu_time, delta);
#endif        
    }

    return 0;
}
`)

func bpfassetsPerf_eventPerf_eventCBytes() ([]byte, error) {
	return _bpfassetsPerf_eventPerf_eventC, nil
}

func bpfassetsPerf_eventPerf_eventC() (*asset, error) {
	bytes, err := bpfassetsPerf_eventPerf_eventCBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "bpfassets/perf_event/perf_event.c", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

// Asset loads and returns the asset for the given name.
// It returns an error if the asset could not be found or
// could not be loaded.
func Asset(name string) ([]byte, error) {
	cannonicalName := strings.Replace(name, "\\", "/", -1)
	if f, ok := _bindata[cannonicalName]; ok {
		a, err := f()
		if err != nil {
			return nil, fmt.Errorf("Asset %s can't read by error: %v", name, err)
		}
		return a.bytes, nil
	}
	return nil, fmt.Errorf("Asset %s not found", name)
}

// MustAsset is like Asset but panics when Asset would return an error.
// It simplifies safe initialization of global variables.
func MustAsset(name string) []byte {
	a, err := Asset(name)
	if err != nil {
		panic("asset: Asset(" + name + "): " + err.Error())
	}

	return a
}

// AssetInfo loads and returns the asset info for the given name.
// It returns an error if the asset could not be found or
// could not be loaded.
func AssetInfo(name string) (os.FileInfo, error) {
	cannonicalName := strings.Replace(name, "\\", "/", -1)
	if f, ok := _bindata[cannonicalName]; ok {
		a, err := f()
		if err != nil {
			return nil, fmt.Errorf("AssetInfo %s can't read by error: %v", name, err)
		}
		return a.info, nil
	}
	return nil, fmt.Errorf("AssetInfo %s not found", name)
}

// AssetNames returns the names of the assets.
func AssetNames() []string {
	names := make([]string, 0, len(_bindata))
	for name := range _bindata {
		names = append(names, name)
	}
	return names
}

// _bindata is a table, holding each asset generator, mapped to its name.
var _bindata = map[string]func() (*asset, error){
	"bpfassets/perf_event/perf_event.c": bpfassetsPerf_eventPerf_eventC,
}

// AssetDir returns the file names below a certain
// directory embedded in the file by go-bindata.
// For example if you run go-bindata on data/... and data contains the
// following hierarchy:
//
//	data/
//	  foo.txt
//	  img/
//	    a.png
//	    b.png
//
// then AssetDir("data") would return []string{"foo.txt", "img"}
// AssetDir("data/img") would return []string{"a.png", "b.png"}
// AssetDir("foo.txt") and AssetDir("notexist") would return an error
// AssetDir("") will return []string{"data"}.
func AssetDir(name string) ([]string, error) {
	node := _bintree
	if len(name) != 0 {
		cannonicalName := strings.Replace(name, "\\", "/", -1)
		pathList := strings.Split(cannonicalName, "/")
		for _, p := range pathList {
			node = node.Children[p]
			if node == nil {
				return nil, fmt.Errorf("Asset %s not found", name)
			}
		}
	}
	if node.Func != nil {
		return nil, fmt.Errorf("Asset %s not found", name)
	}
	rv := make([]string, 0, len(node.Children))
	for childName := range node.Children {
		rv = append(rv, childName)
	}
	return rv, nil
}

type bintree struct {
	Func     func() (*asset, error)
	Children map[string]*bintree
}

var _bintree = &bintree{nil, map[string]*bintree{
	"bpfassets": {nil, map[string]*bintree{
		"perf_event": {nil, map[string]*bintree{
			"perf_event.c": {bpfassetsPerf_eventPerf_eventC, map[string]*bintree{}},
		}},
	}},
}}

// RestoreAsset restores an asset under the given directory
func RestoreAsset(dir, name string) error {
	data, err := Asset(name)
	if err != nil {
		return err
	}
	info, err := AssetInfo(name)
	if err != nil {
		return err
	}
	err = os.MkdirAll(_filePath(dir, filepath.Dir(name)), os.FileMode(0755))
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(_filePath(dir, name), data, info.Mode())
	if err != nil {
		return err
	}
	err = os.Chtimes(_filePath(dir, name), info.ModTime(), info.ModTime())
	if err != nil {
		return err
	}
	return nil
}

// RestoreAssets restores an asset under the given directory recursively
func RestoreAssets(dir, name string) error {
	children, err := AssetDir(name)
	// File
	if err != nil {
		return RestoreAsset(dir, name)
	}
	// Dir
	for _, child := range children {
		err = RestoreAssets(dir, filepath.Join(name, child))
		if err != nil {
			return err
		}
	}
	return nil
}

func _filePath(dir, name string) string {
	cannonicalName := strings.Replace(name, "\\", "/", -1)
	return filepath.Join(append([]string{dir}, strings.Split(cannonicalName, "/")...)...)
}
