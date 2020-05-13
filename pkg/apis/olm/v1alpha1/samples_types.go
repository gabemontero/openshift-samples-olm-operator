package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// SamplesSpec defines the desired state of Samples
type SamplesSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book-v1.book.kubebuilder.io/beyond_basics/generating_crd.html
	ImagestreamURLs    []string `json:"imagestreamurls,omitempty"`
	TemplateURLs       []string `json:"templateurls,omitempty"`
	UpdateImagestreams bool     `json:"updateimagestreams,omitempty"`
	UpdateTemplates    bool     `json:"updatetemplates,omitempty"`
	RetryFailedImports bool     `json:"retryfailedimports,omitempty"`
	RelistInterval     int      `json:"relistinterval,omitempty"`
}

// SamplesStatus defines the observed state of Samples
type SamplesStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book-v1.book.kubebuilder.io/beyond_basics/generating_crd.html
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Samples is the Schema for the samples API
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=samples,scope=Namespaced
type Samples struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SamplesSpec   `json:"spec,omitempty"`
	Status SamplesStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// SamplesList contains a list of Samples
type SamplesList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Samples `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Samples{}, &SamplesList{})
}
