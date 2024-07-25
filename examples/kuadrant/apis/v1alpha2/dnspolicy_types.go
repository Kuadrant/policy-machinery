package v1alpha2

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	gwapiv1alpha2 "sigs.k8s.io/gateway-api/apis/v1alpha2"

	"github.com/kuadrant/policy-machinery/machinery"

	kuadrantapis "github.com/kuadrant/policy-machinery/examples/kuadrant/apis"
)

var (
	DNSPolicyKind       = schema.GroupKind{Group: SchemeGroupVersion.Group, Kind: "DNSPolicy"}
	DNSPoliciesResource = SchemeGroupVersion.WithResource("dnspolicies")
)

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:metadata:labels="gateway.networking.k8s.io/policy=inherited"
// +kubebuilder:printcolumn:name="Accepted",type=string,JSONPath=`.status.conditions[?(@.type=="Accepted")].status`,description="DNSPolicy Accepted",priority=2
// +kubebuilder:printcolumn:name="Enforced",type=string,JSONPath=`.status.conditions[?(@.type=="Enforced")].status`,description="DNSPolicy Enforced",priority=2
// +kubebuilder:printcolumn:name="TargetKind",type="string",JSONPath=".spec.targetRef.kind",description="Kind of the object to which the policy aaplies",priority=2
// +kubebuilder:printcolumn:name="TargetName",type="string",JSONPath=".spec.targetRef.name",description="Name of the object to which the policy applies",priority=2
// +kubebuilder:printcolumn:name="TargetSection",type="string",JSONPath=".spec.targetRef.sectionName",description="Name of the section within the object to which the policy applies ",priority=2
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// DNSPolicy enables automatic cloud DNS configuration for Gateway API objects.
type DNSPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DNSPolicySpec   `json:"spec,omitempty"`
	Status DNSPolicyStatus `json:"status,omitempty"`
}

var _ machinery.Policy = &DNSPolicy{}

func (p *DNSPolicy) GetNamespace() string {
	return p.Namespace
}

func (p *DNSPolicy) GetName() string {
	return p.Name
}

func (p *DNSPolicy) GetURL() string {
	return machinery.UrlFromObject(p)
}

func (p *DNSPolicy) GetTargetRefs() []machinery.PolicyTargetReference {
	return []machinery.PolicyTargetReference{
		machinery.LocalPolicyTargetReferenceWithSectionName{
			LocalPolicyTargetReferenceWithSectionName: p.Spec.TargetRef,
			PolicyNamespace: p.Namespace,
		},
	}
}

func (p *DNSPolicy) GetMergeStrategy() machinery.MergeStrategy {
	return machinery.DefaultMergeStrategy
}

func (p *DNSPolicy) Merge(other machinery.Policy) machinery.Policy {
	source, ok := other.(*DNSPolicy)
	if !ok {
		return p
	}
	return source.GetMergeStrategy()(source, p)
}

var _ kuadrantapis.MergeablePolicy = &DNSPolicy{}

func (p *DNSPolicy) Empty() bool {
	return false
}

func (p *DNSPolicy) Rules() map[string]any {
	return nil
}

func (p *DNSPolicy) SetRules(_ map[string]any) {}

// DNSPolicySpec defines the desired state of DNSPolicy
// +kubebuilder:validation:XValidation:rule="!(self.routingStrategy == 'loadbalanced' && !has(self.loadBalancing))",message="spec.loadBalancing is a required field when spec.routingStrategy == 'loadbalanced'"
type DNSPolicySpec struct {
	// Reference to the object to which this policy applies.
	// +kubebuilder:validation:XValidation:rule="self.group == 'gateway.networking.k8s.io'",message="Invalid targetRef.group. The only supported value is 'gateway.networking.k8s.io'"
	// +kubebuilder:validation:XValidation:rule="self.kind == 'Gateway'",message="Invalid targetRef.kind. The only supported values are 'Gateway'"
	TargetRef gwapiv1alpha2.LocalPolicyTargetReferenceWithSectionName `json:"targetRef"`

	// +optional
	HealthCheck *HealthCheckSpec `json:"healthCheck,omitempty"`

	// +optional
	LoadBalancing *LoadBalancingSpec `json:"loadBalancing,omitempty"`

	// +kubebuilder:validation:Enum=simple;loadbalanced
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="RoutingStrategy is immutable"
	// +kubebuilder:default=loadbalanced
	RoutingStrategy RoutingStrategy `json:"routingStrategy"`
}

// HealthCheckSpec configures health checks in the DNS provider.
// By default, this health check will be applied to each unique DNS A Record for
// the listeners assigned to the target gateway
type HealthCheckSpec struct {
	// Endpoint is the path to append to the host to reach the expected health check.
	// For example "/" or "/healthz" are common
	// +kubebuilder:example:=/
	Endpoint string `json:"endpoint"`
	// Port to connect to the host on
	// +kubebuilder:validation:Minimum:=1
	Port int `json:"port"`
	// Protocol to use when connecting to the host, valid values are "HTTP" or "HTTPS"
	// +kubebuilder:validation:Enum:=HTTP;HTTPS
	Protocol string `json:"protocol"`
	// FailureThreshold is a limit of consecutive failures that must occur for a host
	// to be considered unhealthy
	// +kubebuilder:validation:Minimum:=1
	FailureThreshold int `json:"failureThreshold"`
}

type LoadBalancingSpec struct {
	Weighted LoadBalancingWeighted `json:"weighted"`

	Geo LoadBalancingGeo `json:"geo"`
}

type LoadBalancingWeighted struct {
	// defaultWeight is the record weight to use when no other can be determined for a dns target cluster.
	//
	// The maximum value accepted is determined by the target dns provider, please refer to the appropriate docs below.
	//
	// Route53: https://docs.aws.amazon.com/Route53/latest/DeveloperGuide/routing-policy-weighted.html
	DefaultWeight Weight `json:"defaultWeight"`

	// custom list of custom weight selectors.
	// +optional
	Custom []*CustomWeight `json:"custom,omitempty"`
}

// +kubebuilder:validation:Minimum=0
type Weight int

type CustomWeight struct {
	// Label selector to match resource storing custom weight attribute values e.g. kuadrant.io/lb-attribute-custom-weight: AWS.
	Selector *metav1.LabelSelector `json:"selector"`

	// The weight value to apply when the selector matches.
	Weight Weight `json:"weight"`
}

type LoadBalancingGeo struct {
	// defaultGeo is the country/continent/region code to use when no other can be determined for a dns target cluster.
	//
	// The values accepted are determined by the target dns provider, please refer to the appropriate docs below.
	//
	// Route53: https://docs.aws.amazon.com/Route53/latest/DeveloperGuide/resource-record-sets-values-geo.html
	// Google: https://cloud.google.com/compute/docs/regions-zones
	// +kubebuilder:validation:MinLength=2
	DefaultGeo string `json:"defaultGeo"`
}

type RoutingStrategy string

type DNSPolicyStatus struct {
	// ObservedGeneration reflects the generation of the most recently observed spec.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Represents the observations of a foo's current state.
	// Known .status.conditions.type are: "Available"
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"`
}

func (s *DNSPolicyStatus) GetConditions() []metav1.Condition {
	return s.Conditions
}

//+kubebuilder:object:root=true

// DNSPolicyList contains a list of DNSPolicy
type DNSPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DNSPolicy `json:"items"`
}
