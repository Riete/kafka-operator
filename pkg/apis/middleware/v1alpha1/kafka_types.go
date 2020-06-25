package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	KafkaExporterImage string = "riet/kafka-exporter:1.2.0"
	KafkaExporterPort  int32  = 9308
	REPOSITORY         string = "riet/kafka"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// KafkaSpec defines the desired state of Kafka
type KafkaSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book-v1.book.kubebuilder.io/beyond_basics/generating_crd.html
	Tag               string                      `json:"tag"`
	KafkaVersion      string                      `json:"kafkaVersion"`
	Servers           int32                       `json:"servers"`
	Resources         corev1.ResourceRequirements `json:"resources,omitempty"`
	StorageClass      string                      `json:"storageClass,omitempty"`
	StorageSize       string                      `json:"storageSize,omitempty"`
	Metrics           bool                        `json:"metrics,omitempty"`
	Heap              string                      `json:"heap,omitempty"`
	Zookeeper         string                      `json:"zookeeper"`
	NumPartitions     int                         `json:"numPartitions"`
	LogRetentionHours int                         `json:"logRetentionHours"`
}

// KafkaStatus defines the observed state of Kafka
type KafkaStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book-v1.book.kubebuilder.io/beyond_basics/generating_crd.html
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Kafka is the Schema for the kafkas API
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=kafkas,scope=Namespaced
type Kafka struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   KafkaSpec   `json:"spec,omitempty"`
	Status KafkaStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// KafkaList contains a list of Kafka
type KafkaList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Kafka `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Kafka{}, &KafkaList{})
}
