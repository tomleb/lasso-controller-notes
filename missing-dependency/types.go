package main

// Dummy types loosely modelled after Rancher's Cluster and
// ClusterRegistrationToken (CRT). They are hand-written (no codegen) to keep
// the example small: we only need enough to satisfy runtime.Object so that
// lasso can decode them and wrangler's generic controllers can use them.

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	SchemeGroupVersion = schema.GroupVersion{Group: "example.com", Version: "v1"}

	ClusterGVK                       = SchemeGroupVersion.WithKind("Cluster")
	ClusterRegistrationTokenGVK      = SchemeGroupVersion.WithKind("ClusterRegistrationToken")
	clusterResource                  = "clusters"
	clusterRegistrationTokenResource = "clusterregistrationtokens"
)

func addToScheme(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(SchemeGroupVersion,
		&Cluster{}, &ClusterList{},
		&ClusterRegistrationToken{}, &ClusterRegistrationTokenList{},
	)
	metav1.AddToGroupVersion(scheme, SchemeGroupVersion)
	return nil
}

// Cluster is cluster-scoped.
type Cluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
}

type ClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Cluster `json:"items"`
}

// ClusterRegistrationToken is namespaced and points to the Cluster it belongs
// to with .spec.clusterName. It can only be "ready" once that Cluster exists.
type ClusterRegistrationToken struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ClusterRegistrationTokenSpec   `json:"spec"`
	Status ClusterRegistrationTokenStatus `json:"status"`
}

type ClusterRegistrationTokenSpec struct {
	ClusterName string `json:"clusterName"`
}

type ClusterRegistrationTokenStatus struct {
	Token string `json:"token,omitempty"`
}

type ClusterRegistrationTokenList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterRegistrationToken `json:"items"`
}

// Hand-written deepcopy functions. These would normally be generated.

func (in *Cluster) DeepCopy() *Cluster {
	out := *in
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	return &out
}

func (in *Cluster) DeepCopyObject() runtime.Object { return in.DeepCopy() }

func (in *ClusterList) DeepCopyObject() runtime.Object {
	out := *in
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	out.Items = make([]Cluster, len(in.Items))
	for i := range in.Items {
		out.Items[i] = *in.Items[i].DeepCopy()
	}
	return &out
}

func (in *ClusterRegistrationToken) DeepCopy() *ClusterRegistrationToken {
	out := *in
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	return &out
}

func (in *ClusterRegistrationToken) DeepCopyObject() runtime.Object { return in.DeepCopy() }

func (in *ClusterRegistrationTokenList) DeepCopyObject() runtime.Object {
	out := *in
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	out.Items = make([]ClusterRegistrationToken, len(in.Items))
	for i := range in.Items {
		out.Items[i] = *in.Items[i].DeepCopy()
	}
	return &out
}
