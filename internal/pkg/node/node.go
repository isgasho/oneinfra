/*
Copyright 2020 Rafael Fernández López <ereslibre@ereslibre.es>

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

package node

import (
	"fmt"

	"github.com/pkg/errors"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"

	clusterv1alpha1 "oneinfra.ereslibre.es/m/apis/cluster/v1alpha1"
	"oneinfra.ereslibre.es/m/internal/pkg/infra"
)

var (
	components = []ComponentType{
		KubeAPIServerComponent,
		KubeControllerManagerComponent,
		KubeSchedulerComponent,
	}
)

// Node represents a Control Plane node
type Node struct {
	Name           string
	HypervisorName string
	ClusterName    string
	hypervisor     *infra.Hypervisor
}

// List represents a list of nodes
type List []*Node

// NewNodeWithRandomHypervisor creates a node with a random hypervisor from the provided hypervisorList
func NewNodeWithRandomHypervisor(nodeName, clusterName string, hypervisorList infra.HypervisorList) *Node {
	hypervisorSample := hypervisorList.Sample()
	return &Node{
		Name:           nodeName,
		HypervisorName: hypervisorSample.Name,
		ClusterName:    clusterName,
		hypervisor:     hypervisorSample,
	}
}

// NewNodeFromv1alpha1 returns a node based on a versioned node
func NewNodeFromv1alpha1(node *clusterv1alpha1.Node) (*Node, error) {
	return &Node{
		Name:           node.ObjectMeta.Name,
		HypervisorName: node.Spec.Hypervisor,
		ClusterName:    node.Spec.Cluster,
	}, nil
}

// NewNodeWithHypervisorFromv1alpha1 returns a node based on a versioned node and an hypervisor
func NewNodeWithHypervisorFromv1alpha1(node *clusterv1alpha1.Node, hypervisor *infra.Hypervisor) (*Node, error) {
	return &Node{
		Name:           node.ObjectMeta.Name,
		HypervisorName: node.Spec.Hypervisor,
		ClusterName:    node.Spec.Cluster,
		hypervisor:     hypervisor,
	}, nil
}

// Component returns a component of type componentType
func (node *Node) Component(componentType ComponentType) (Component, error) {
	switch componentType {
	case KubeAPIServerComponent:
		return &KubeAPIServer{}, nil
	case KubeControllerManagerComponent:
		return &KubeControllerManager{}, nil
	case KubeSchedulerComponent:
		return &KubeScheduler{}, nil
	default:
		return nil, errors.Errorf("unknown component: %d", componentType)
	}
}

// Reconcile reconciles the node
func (node *Node) Reconcile() error {
	if node.hypervisor == nil {
		return errors.Errorf("node %q is missing an hypervisor", node.Name)
	}
	for _, componentType := range components {
		component, err := node.Component(componentType)
		if err != nil {
			return err
		}
		if err := component.Reconcile(node.hypervisor); err != nil {
			return err
		}
	}
	return nil
}

// Export exports the node to a versioned node
func (node *Node) Export() *clusterv1alpha1.Node {
	return &clusterv1alpha1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: node.Name,
		},
		Spec: clusterv1alpha1.NodeSpec{
			Hypervisor: node.HypervisorName,
			Cluster:    node.ClusterName,
			Role:       clusterv1alpha1.ControlPlaneRole,
		},
	}
}

// Specs returns the versioned specs of this node
func (node *Node) Specs() (string, error) {
	scheme := runtime.NewScheme()
	if err := clusterv1alpha1.AddToScheme(scheme); err != nil {
		return "", err
	}
	info, _ := runtime.SerializerInfoForMediaType(serializer.NewCodecFactory(scheme).SupportedMediaTypes(), runtime.ContentTypeYAML)
	encoder := serializer.NewCodecFactory(scheme).EncoderForVersion(info.Serializer, clusterv1alpha1.GroupVersion)
	nodeObject := node.Export()
	if encodedNode, err := runtime.Encode(encoder, nodeObject); err == nil {
		return string(encodedNode), nil
	}
	return "", errors.Errorf("could not encode node %q", node.Name)
}

// Specs returns the versioned specs of all nodes in this list
func (list List) Specs() (string, error) {
	res := ""
	for _, node := range list {
		nodeSpec, err := node.Specs()
		if err != nil {
			continue
		}
		res += fmt.Sprintf("---\n%s", nodeSpec)
	}
	return res, nil
}
