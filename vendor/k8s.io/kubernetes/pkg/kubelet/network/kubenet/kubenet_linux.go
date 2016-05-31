// +build linux

/*
Copyright 2014 The Kubernetes Authors All rights reserved.

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

package kubenet

import (
	"fmt"
	"net"

	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netlink/nl"

	"github.com/appc/cni/libcni"
	cnitypes "github.com/appc/cni/pkg/types"
	"github.com/golang/glog"
	"k8s.io/kubernetes/pkg/apis/componentconfig"
	kubecontainer "k8s.io/kubernetes/pkg/kubelet/container"
	"k8s.io/kubernetes/pkg/kubelet/network"
	"k8s.io/kubernetes/pkg/util/bandwidth"
	utilexec "k8s.io/kubernetes/pkg/util/exec"
	utilsets "k8s.io/kubernetes/pkg/util/sets"
	utilsysctl "k8s.io/kubernetes/pkg/util/sysctl"
)

const (
	KubenetPluginName = "kubenet"
	BridgeName        = "cbr0"
	DefaultCNIDir     = "/opt/cni/bin"

	sysctlBridgeCallIptables = "net/bridge/bridge-nf-call-iptables"
)

type kubenetNetworkPlugin struct {
	network.NoopNetworkPlugin

	host        network.Host
	netConfig   *libcni.NetworkConfig
	loConfig    *libcni.NetworkConfig
	cniConfig   *libcni.CNIConfig
	shaper      bandwidth.BandwidthShaper
	podCIDRs    map[kubecontainer.ContainerID]string
	MTU         int
	mu          sync.Mutex //Mutex for protecting podCIDRs map and netConfig
	execer      utilexec.Interface
	nsenterPath string
	hairpinMode componentconfig.HairpinMode
}

func NewPlugin() network.NetworkPlugin {
	return &kubenetNetworkPlugin{
		podCIDRs: make(map[kubecontainer.ContainerID]string),
		MTU:      1460,
		execer:   utilexec.New(),
	}
}

func (plugin *kubenetNetworkPlugin) Init(host network.Host, hairpinMode componentconfig.HairpinMode) error {
	plugin.host = host
	plugin.hairpinMode = hairpinMode
	plugin.cniConfig = &libcni.CNIConfig{
		Path: []string{DefaultCNIDir},
	}

	if link, err := findMinMTU(); err == nil {
		plugin.MTU = link.MTU
		glog.V(5).Infof("Using interface %s MTU %d as bridge MTU", link.Name, link.MTU)
	} else {
		glog.Warningf("Failed to find default bridge MTU: %v", err)
	}

	// Since this plugin uses a Linux bridge, set bridge-nf-call-iptables=1
	// is necessary to ensure kube-proxy functions correctly.
	//
	// This will return an error on older kernel version (< 3.18) as the module
	// was built-in, we simply ignore the error here. A better thing to do is
	// to check the kernel version in the future.
	plugin.execer.Command("modprobe", "br-netfilter").CombinedOutput()
	err := utilsysctl.SetSysctl(sysctlBridgeCallIptables, 1)
	if err != nil {
		glog.Warningf("can't set sysctl %s: %v", sysctlBridgeCallIptables, err)
	}

	plugin.loConfig, err = libcni.ConfFromBytes([]byte(`{
  "cniVersion": "0.1.0",
  "name": "kubenet-loopback",
  "type": "loopback"
}`))
	if err != nil {
		return fmt.Errorf("Failed to generate loopback config: %v", err)
	}

	return nil
}

func findMinMTU() (*net.Interface, error) {
	intfs, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	mtu := 999999
	defIntfIndex := -1
	for i, intf := range intfs {
		if ((intf.Flags & net.FlagUp) != 0) && (intf.Flags&(net.FlagLoopback|net.FlagPointToPoint) == 0) {
			if intf.MTU < mtu {
				mtu = intf.MTU
				defIntfIndex = i
			}
		}
	}

	if mtu >= 999999 || mtu < 576 || defIntfIndex < 0 {
		return nil, fmt.Errorf("no suitable interface: %v", BridgeName)
	}

	return &intfs[defIntfIndex], nil
}

const NET_CONFIG_TEMPLATE = `{
  "cniVersion": "0.1.0",
  "name": "kubenet",
  "type": "bridge",
  "bridge": "%s",
  "mtu": %d,
  "addIf": "%s",
  "isGateway": true,
  "ipMasq": true,
  "ipam": {
    "type": "host-local",
    "subnet": "%s",
    "gateway": "%s",
    "routes": [
      { "dst": "0.0.0.0/0" }
    ]
  }
}`

func (plugin *kubenetNetworkPlugin) Event(name string, details map[string]interface{}) {
	if name != network.NET_PLUGIN_EVENT_POD_CIDR_CHANGE {
		return
	}

	plugin.mu.Lock()
	defer plugin.mu.Unlock()

	podCIDR, ok := details[network.NET_PLUGIN_EVENT_POD_CIDR_CHANGE_DETAIL_CIDR].(string)
	if !ok {
		glog.Warningf("%s event didn't contain pod CIDR", network.NET_PLUGIN_EVENT_POD_CIDR_CHANGE)
		return
	}

	if plugin.netConfig != nil {
		glog.V(5).Infof("Ignoring subsequent pod CIDR update to %s", podCIDR)
		return
	}

	glog.V(5).Infof("PodCIDR is set to %q", podCIDR)
	_, cidr, err := net.ParseCIDR(podCIDR)
	if err == nil {
		// Set bridge address to first address in IPNet
		cidr.IP.To4()[3] += 1

		json := fmt.Sprintf(NET_CONFIG_TEMPLATE, BridgeName, plugin.MTU, network.DefaultInterfaceName, podCIDR, cidr.IP.String())
		glog.V(2).Infof("CNI network config set to %v", json)
		plugin.netConfig, err = libcni.ConfFromBytes([]byte(json))
		if err == nil {
			glog.V(5).Infof("CNI network config:\n%s", json)

			// Ensure cbr0 has no conflicting addresses; CNI's 'bridge'
			// plugin will bail out if the bridge has an unexpected one
			plugin.clearBridgeAddressesExcept(cidr.IP.String())
		}
	}

	if err != nil {
		glog.Warningf("Failed to generate CNI network config: %v", err)
	}
}

func (plugin *kubenetNetworkPlugin) clearBridgeAddressesExcept(keep string) {
	bridge, err := netlink.LinkByName(BridgeName)
	if err != nil {
		return
	}

	addrs, err := netlink.AddrList(bridge, syscall.AF_INET)
	if err != nil {
		return
	}

	for _, addr := range addrs {
		if addr.IPNet.String() != keep {
			glog.V(5).Infof("Removing old address %s from %s", addr.IPNet.String(), BridgeName)
			netlink.AddrDel(bridge, &addr)
		}
	}
}

// ensureBridgeTxQueueLen() ensures that the bridge interface's TX queue
// length is greater than zero.  Due to a CNI <= 0.3.0 'bridge' plugin bug,
// the bridge is initially created with a TX queue length of 0, which gets
// used as the packet limit for FIFO traffic shapers, which drops packets.
// TODO: remove when we can depend on a fixed CNI
func (plugin *kubenetNetworkPlugin) ensureBridgeTxQueueLen() {
	bridge, err := netlink.LinkByName(BridgeName)
	if err != nil {
		return
	}

	if bridge.Attrs().TxQLen > 0 {
		return
	}

	req := nl.NewNetlinkRequest(syscall.RTM_NEWLINK, syscall.NLM_F_ACK)
	msg := nl.NewIfInfomsg(syscall.AF_UNSPEC)
	req.AddData(msg)

	nameData := nl.NewRtAttr(syscall.IFLA_IFNAME, nl.ZeroTerminated(BridgeName))
	req.AddData(nameData)

	qlen := nl.NewRtAttr(syscall.IFLA_TXQLEN, nl.Uint32Attr(1000))
	req.AddData(qlen)

	_, err = req.Execute(syscall.NETLINK_ROUTE, 0)
	if err != nil {
		glog.V(5).Infof("Failed to set bridge tx queue length: %v", err)
	}
}

func (plugin *kubenetNetworkPlugin) Name() string {
	return KubenetPluginName
}

func (plugin *kubenetNetworkPlugin) Capabilities() utilsets.Int {
	return utilsets.NewInt(network.NET_PLUGIN_CAPABILITY_SHAPING)
}

func (plugin *kubenetNetworkPlugin) SetUpPod(namespace string, name string, id kubecontainer.ContainerID) error {
	plugin.mu.Lock()
	defer plugin.mu.Unlock()

	start := time.Now()
	defer func() {
		glog.V(4).Infof("SetUpPod took %v for %s/%s", time.Since(start), namespace, name)
	}()

	pod, ok := plugin.host.GetPodByName(namespace, name)
	if !ok {
		return fmt.Errorf("pod %q cannot be found", name)
	}
	ingress, egress, err := bandwidth.ExtractPodBandwidthResources(pod.Annotations)
	if err != nil {
		return fmt.Errorf("Error reading pod bandwidth annotations: %v", err)
	}

	if err := plugin.Status(); err != nil {
		return fmt.Errorf("Kubenet cannot SetUpPod: %v", err)
	}

	// Bring up container loopback interface
	if _, err := plugin.addContainerToNetwork(plugin.loConfig, "lo", namespace, name, id); err != nil {
		return err
	}

	// Hook container up with our bridge
	res, err := plugin.addContainerToNetwork(plugin.netConfig, network.DefaultInterfaceName, namespace, name, id)
	if err != nil {
		return err
	}
	if res.IP4 == nil || res.IP4.IP.String() == "" {
		return fmt.Errorf("CNI plugin reported no IPv4 address for container %v.", id)
	}
	plugin.podCIDRs[id] = res.IP4.IP.String()

	// Put the container bridge into promiscuous mode to force it to accept hairpin packets.
	// TODO: Remove this once the kernel bug (#20096) is fixed.
	// TODO: check and set promiscuous mode with netlink once vishvananda/netlink supports it
	if plugin.hairpinMode == componentconfig.PromiscuousBridge {
		output, err := plugin.execer.Command("ip", "link", "show", "dev", BridgeName).CombinedOutput()
		if err != nil || strings.Index(string(output), "PROMISC") < 0 {
			_, err := plugin.execer.Command("ip", "link", "set", BridgeName, "promisc", "on").CombinedOutput()
			if err != nil {
				return fmt.Errorf("Error setting promiscuous mode on %s: %v", BridgeName, err)
			}
		}
	}

	// The first SetUpPod call creates the bridge; ensure shaping is enabled
	if plugin.shaper == nil {
		plugin.shaper = bandwidth.NewTCShaper(BridgeName)
		if plugin.shaper == nil {
			return fmt.Errorf("Failed to create bandwidth shaper!")
		}
		plugin.ensureBridgeTxQueueLen()
		plugin.shaper.ReconcileInterface()
	}

	if egress != nil || ingress != nil {
		ipAddr, _, _ := net.ParseCIDR(plugin.podCIDRs[id])
		if err = plugin.shaper.ReconcileCIDR(fmt.Sprintf("%s/32", ipAddr.String()), egress, ingress); err != nil {
			return fmt.Errorf("Failed to add pod to shaper: %v", err)
		}
	}

	return nil
}

func (plugin *kubenetNetworkPlugin) TearDownPod(namespace string, name string, id kubecontainer.ContainerID) error {
	plugin.mu.Lock()
	defer plugin.mu.Unlock()

	start := time.Now()
	defer func() {
		glog.V(4).Infof("TearDownPod took %v for %s/%s", time.Since(start), namespace, name)
	}()

	if plugin.netConfig == nil {
		return fmt.Errorf("Kubenet needs a PodCIDR to tear down pods")
	}

	// no cached CIDR is Ok during teardown
	if cidr, ok := plugin.podCIDRs[id]; ok {
		glog.V(5).Infof("Removing pod CIDR %s from shaper", cidr)
		// shaper wants /32
		if addr, _, err := net.ParseCIDR(cidr); err != nil {
			if err = plugin.shaper.Reset(fmt.Sprintf("%s/32", addr.String())); err != nil {
				glog.Warningf("Failed to remove pod CIDR %s from shaper: %v", cidr, err)
			}
		}
	}
	if err := plugin.delContainerFromNetwork(plugin.netConfig, network.DefaultInterfaceName, namespace, name, id); err != nil {
		return err
	}
	delete(plugin.podCIDRs, id)

	return nil
}

// TODO: Use the addToNetwork function to obtain the IP of the Pod. That will assume idempotent ADD call to the plugin.
// Also fix the runtime's call to Status function to be done only in the case that the IP is lost, no need to do periodic calls
func (plugin *kubenetNetworkPlugin) GetPodNetworkStatus(namespace string, name string, id kubecontainer.ContainerID) (*network.PodNetworkStatus, error) {
	plugin.mu.Lock()
	defer plugin.mu.Unlock()
	// Assuming the ip of pod does not change. Try to retrieve ip from kubenet map first.
	if cidr, ok := plugin.podCIDRs[id]; ok {
		ip, _, err := net.ParseCIDR(strings.Trim(cidr, "\n"))
		if err == nil {
			return &network.PodNetworkStatus{IP: ip}, nil
		}
	}

	netnsPath, err := plugin.host.GetRuntime().GetNetNS(id)
	if err != nil {
		return nil, fmt.Errorf("Kubenet failed to retrieve network namespace path: %v", err)
	}
	nsenterPath, err := plugin.getNsenterPath()
	if err != nil {
		return nil, err
	}
	// Try to retrieve ip inside container network namespace
	output, err := plugin.execer.Command(nsenterPath, fmt.Sprintf("--net=%s", netnsPath), "-F", "--",
		"ip", "-o", "-4", "addr", "show", "dev", network.DefaultInterfaceName).CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("Unexpected command output %s with error: %v", output, err)
	}
	fields := strings.Fields(string(output))
	if len(fields) < 4 {
		return nil, fmt.Errorf("Unexpected command output %s ", output)
	}
	ip, _, err := net.ParseCIDR(fields[3])
	if err != nil {
		return nil, fmt.Errorf("Kubenet failed to parse ip from output %s due to %v", output, err)
	}
	plugin.podCIDRs[id] = ip.String()
	return &network.PodNetworkStatus{IP: ip}, nil
}

func (plugin *kubenetNetworkPlugin) Status() error {
	// Can't set up pods if we don't have a PodCIDR yet
	if plugin.netConfig == nil {
		return fmt.Errorf("Kubenet does not have netConfig. This is most likely due to lack of PodCIDR")
	}
	return nil
}

func (plugin *kubenetNetworkPlugin) buildCNIRuntimeConf(ifName string, id kubecontainer.ContainerID) (*libcni.RuntimeConf, error) {
	netnsPath, err := plugin.host.GetRuntime().GetNetNS(id)
	if err != nil {
		return nil, fmt.Errorf("Kubenet failed to retrieve network namespace path: %v", err)
	}

	return &libcni.RuntimeConf{
		ContainerID: id.ID,
		NetNS:       netnsPath,
		IfName:      ifName,
	}, nil
}

func (plugin *kubenetNetworkPlugin) addContainerToNetwork(config *libcni.NetworkConfig, ifName, namespace, name string, id kubecontainer.ContainerID) (*cnitypes.Result, error) {
	rt, err := plugin.buildCNIRuntimeConf(ifName, id)
	if err != nil {
		return nil, fmt.Errorf("Error building CNI config: %v", err)
	}

	glog.V(3).Infof("Adding %s/%s to '%s' with CNI '%s' plugin and runtime: %+v", namespace, name, config.Network.Name, config.Network.Type, rt)
	res, err := plugin.cniConfig.AddNetwork(config, rt)
	if err != nil {
		return nil, fmt.Errorf("Error adding container to network: %v", err)
	}
	return res, nil
}

func (plugin *kubenetNetworkPlugin) delContainerFromNetwork(config *libcni.NetworkConfig, ifName, namespace, name string, id kubecontainer.ContainerID) error {
	rt, err := plugin.buildCNIRuntimeConf(ifName, id)
	if err != nil {
		return fmt.Errorf("Error building CNI config: %v", err)
	}

	glog.V(3).Infof("Removing %s/%s from '%s' with CNI '%s' plugin and runtime: %+v", namespace, name, config.Network.Name, config.Network.Type, rt)
	if err := plugin.cniConfig.DelNetwork(config, rt); err != nil {
		return fmt.Errorf("Error removing container from network: %v", err)
	}
	return nil
}

func (plugin *kubenetNetworkPlugin) getNsenterPath() (string, error) {
	if plugin.nsenterPath == "" {
		nsenterPath, err := plugin.execer.LookPath("nsenter")
		if err != nil {
			return "", err
		}
		plugin.nsenterPath = nsenterPath
	}
	return plugin.nsenterPath, nil
}
