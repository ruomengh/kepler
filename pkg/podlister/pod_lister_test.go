package podlister

import (
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"

	. "github.com/onsi/gomega"

	"github.com/sustainable-computing-io/kepler/pkg/utils"
)

const rhelContainerd = `
13:memory:/system.slice/containerd.service/kubepods-besteffort-pod0043435f_1854_4327_b76b_730f681a781d.slice:cri-containerd:01fd96f7ad292b02a8317cde4ecb8c7ef3cc06ffdd113f13410e0837eb2b2a20`

const rhelContainerdExpected = `13:memory:/system.slice/containerd.service/kubepods-besteffort-pod0043435f_1854_4327_b76b_730f681a781d.slice:cri-containerd:01fd96f7ad292b02a8317cde4ecb8c7ef3cc06ffdd113f13410e0837eb2b2a20`

const rhelDocker = `
11:blkio:/system.slice/docker-c27755f0fa91e81ababc85ef05cb227227a4228da2e5cb2f4999299c89d4ac69.scope/kubelet.slice/kubelet-kubepods.slice/kubelet-kubepods-burstable.slice/kubelet-kubepods-burstable-podd8992f589d8dd12c4342376ccb459375.slice/cri-containerd-ecbcea5cd29afb25ba519715e827cda9e66cd0a914207f49ce0a292a6aa84d66.scope
1:name=systemd:/system.slice/docker-c27755f0fa91e81ababc85ef05cb227227a4228da2e5cb2f4999299c89d4ac69.scope/kubelet.slice/kubelet-kubepods.slice/kubelet-kubepods-burstable.slice/kubelet-kubepods-burstable-podd8992f589d8dd12c4342376ccb459375.slice/cri-containerd-ecbcea5cd29afb25ba519715e827cda9e66cd0a914207f49ce0a292a6aa84d66.scope`

const rhelDockerExpected = `11:blkio:/system.slice/docker-c27755f0fa91e81ababc85ef05cb227227a4228da2e5cb2f4999299c89d4ac69.scope/kubelet.slice/kubelet-kubepods.slice/kubelet-kubepods-burstable.slice/kubelet-kubepods-burstable-podd8992f589d8dd12c4342376ccb459375.slice/cri-containerd-ecbcea5cd29afb25ba519715e827cda9e66cd0a914207f49ce0a292a6aa84d66.scope`

const ubuntuContainerd = `
0::/kubelet.slice/kubelet-kubepods.slice/kubelet-kubepods-besteffort.slice/kubelet-kubepods-besteffort-pod36f20d9d_cbc1_4ebd_b111_536eaa6a332e.slice/cri-containerd-db90aabe3ba00bab92a9bd3f0b4a9face4601651c91d28c02a953a8c81ce2cc4.scope
`

const ubuntuContainerdExpected = `0::/kubelet.slice/kubelet-kubepods.slice/kubelet-kubepods-besteffort.slice/kubelet-kubepods-besteffort-pod36f20d9d_cbc1_4ebd_b111_536eaa6a332e.slice/cri-containerd-db90aabe3ba00bab92a9bd3f0b4a9face4601651c91d28c02a953a8c81ce2cc4.scope`

const ubuntuDocker = `
11:blkio:/kubepods.slice/kubepods-besteffort.slice/kubepods-besteffort-pod481c0ae9_7d40_46dd_b6ca_ba27cb64f87e.slice/docker-28a5e57257f81fcd6d592647dde27e06b53944d58af4fa546ad77a12ce8b41c2.scope`

const ubuntuDockerExpected = `11:blkio:/kubepods.slice/kubepods-besteffort.slice/kubepods-besteffort-pod481c0ae9_7d40_46dd_b6ca_ba27cb64f87e.slice/docker-28a5e57257f81fcd6d592647dde27e06b53944d58af4fa546ad77a12ce8b41c2.scope`

func TestGetPathFromPID(t *testing.T) {
	g := NewWithT(t)

	var testcases = []struct {
		name        string
		contents    string
		expectedRet string
		expectErr   bool
	}{
		{
			name:        "test rhel containerd",
			contents:    rhelContainerd,
			expectedRet: rhelContainerdExpected,
		},
		{
			name:        "test ubuntu containerd",
			contents:    ubuntuContainerd,
			expectedRet: ubuntuContainerdExpected,
		},
		{
			name:        "test rhel docker",
			contents:    rhelDocker,
			expectedRet: rhelDockerExpected,
		},
		{
			name:        "test ubuntu docker",
			contents:    ubuntuDocker,
			expectedRet: ubuntuDockerExpected,
		},
	}
	for _, testcase := range testcases {
		t.Run(testcase.name, func(t *testing.T) {
			p, err := utils.CreateTempFile(testcase.contents)
			_, file := filepath.Split(p)
			g.Expect(err).NotTo(HaveOccurred())
			defer os.Remove(p)

			s := "/tmp/%d"
			d, err := strconv.Atoi(file)
			g.Expect(err).NotTo(HaveOccurred())
			ret, err := getPathFromPID(s, uint64(d))
			if runtime.GOOS == "linux" {
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(ret).To(Equal(testcase.expectedRet))
			}
		})
	}
}
