package v1beta3

import (
	"encoding/json"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	gwapiv1alpha2 "sigs.k8s.io/gateway-api/apis/v1alpha2"

	"github.com/kuadrant/policy-machinery/machinery"

	kuadrantapis "github.com/kuadrant/policy-machinery/examples/kuadrant/apis"
)

var (
	RateLimitPolicyKind       = schema.GroupKind{Group: SchemeGroupVersion.Group, Kind: "RateLimitPolicy"}
	RateLimitPoliciesResource = SchemeGroupVersion.WithResource("ratelimitpolicies")
)

const (
	EqualOperator      WhenConditionOperator = "eq"
	NotEqualOperator   WhenConditionOperator = "neq"
	StartsWithOperator WhenConditionOperator = "startswith"
	EndsWithOperator   WhenConditionOperator = "endswith"
	IncludeOperator    WhenConditionOperator = "incl"
	ExcludeOperator    WhenConditionOperator = "excl"
	MatchesOperator    WhenConditionOperator = "matches"
)

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:metadata:labels="gateway.networking.k8s.io/policy=inherited"
// +kubebuilder:printcolumn:name="Accepted",type=string,JSONPath=`.status.conditions[?(@.type=="Accepted")].status`,description="RateLimitPolicy Accepted",priority=2
// +kubebuilder:printcolumn:name="Enforced",type=string,JSONPath=`.status.conditions[?(@.type=="Enforced")].status`,description="RateLimitPolicy Enforced",priority=2
// +kubebuilder:printcolumn:name="TargetKind",type="string",JSONPath=".spec.targetRef.kind",description="Kind of the object to which the policy aaplies",priority=2
// +kubebuilder:printcolumn:name="TargetName",type="string",JSONPath=".spec.targetRef.name",description="Name of the object to which the policy applies",priority=2
// +kubebuilder:printcolumn:name="TargetSection",type="string",JSONPath=".spec.targetRef.sectionName",description="Name of the section within the object to which the policy applies ",priority=2
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// RateLimitPolicy enables rate limiting for service workloads in a Gateway API network
type RateLimitPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RateLimitPolicySpec   `json:"spec,omitempty"`
	Status RateLimitPolicyStatus `json:"status,omitempty"`
}

var _ machinery.Policy = &RateLimitPolicy{}

func (p *RateLimitPolicy) GetNamespace() string {
	return p.Namespace
}

func (p *RateLimitPolicy) GetName() string {
	return p.Name
}

func (p *RateLimitPolicy) GetLocator() string {
	return machinery.LocatorFromObject(p)
}

func (p *RateLimitPolicy) GetTargetRefs() []machinery.PolicyTargetReference {
	return []machinery.PolicyTargetReference{
		machinery.LocalPolicyTargetReferenceWithSectionName{
			LocalPolicyTargetReferenceWithSectionName: p.Spec.TargetRef,
			PolicyNamespace: p.Namespace,
		},
	}
}

func (p *RateLimitPolicy) GetMergeStrategy() machinery.MergeStrategy {
	if spec := p.Spec.Defaults; spec != nil {
		return kuadrantapis.DefaultsMergeStrategy(spec.Strategy)
	}
	if spec := p.Spec.Overrides; spec != nil {
		return kuadrantapis.OverridesMergeStrategy(spec.Strategy)
	}
	return kuadrantapis.AtomicDefaultsMergeStrategy
}

func (p *RateLimitPolicy) Merge(other machinery.Policy) machinery.Policy {
	source, ok := other.(*RateLimitPolicy)
	if !ok {
		return p
	}
	return source.GetMergeStrategy()(source, p)
}

var _ kuadrantapis.MergeablePolicy = &RateLimitPolicy{}

func (p *RateLimitPolicy) Empty() bool {
	return len(p.Spec.Proper().Limits) == 0
}

func (p *RateLimitPolicy) Rules() map[string]any {
	rules := make(map[string]any)

	for ruleId := range p.Spec.Proper().Limits {
		rules[ruleId] = p.Spec.Proper().Limits[ruleId]
	}

	return rules
}

func (p *RateLimitPolicy) SetRules(rules map[string]any) {
	if len(rules) > 0 && p.Spec.Proper().Limits == nil {
		p.Spec.Proper().Limits = make(map[string]Limit)
	}

	for ruleId := range rules {
		rule := rules[ruleId]
		p.Spec.Proper().Limits[ruleId] = rule.(Limit)
	}
}

// +kubebuilder:validation:XValidation:rule="!(has(self.defaults) && has(self.limits))",message="Implicit and explicit defaults are mutually exclusive"
// +kubebuilder:validation:XValidation:rule="!(has(self.defaults) && has(self.overrides))",message="Overrides and explicit defaults are mutually exclusive"
// +kubebuilder:validation:XValidation:rule="!(has(self.overrides) && has(self.limits))",message="Overrides and implicit defaults are mutually exclusive"
type RateLimitPolicySpec struct {
	// Reference to the object to which this policy applies.
	// +kubebuilder:validation:XValidation:rule="self.group == 'gateway.networking.k8s.io'",message="Invalid targetRef.group. The only supported value is 'gateway.networking.k8s.io'"
	// +kubebuilder:validation:XValidation:rule="self.kind == 'HTTPRoute' || self.kind == 'Gateway'",message="Invalid targetRef.kind. The only supported values are 'HTTPRoute' and 'Gateway'"
	TargetRef gwapiv1alpha2.LocalPolicyTargetReferenceWithSectionName `json:"targetRef"`

	// Rules to apply as defaults. Can be overridden by more specific policiy rules lower in the hierarchy and by less specific policy overrides.
	// Use one of: defaults, overrides, or bare set of policy rules (implicit defaults).
	// +optional
	Defaults *MergeableRateLimitPolicySpec `json:"defaults,omitempty"`

	// Rules to apply as overrides. Override all policy rules lower in the hierarchy. Can be overriden by less specific policy overrides.
	// Use one of: defaults, overrides, or bare set of policy rules (implicit defaults).
	// +optional
	Overrides *MergeableRateLimitPolicySpec `json:"overrides,omitempty"`

	// Bare set of policy rules (implicit defaults).
	// Use one of: defaults, overrides, or bare set of policy rules (implicit defaults).
	RateLimitPolicySpecProper `json:""`
}

// UnmarshalJSON unmarshals the RateLimitPolicySpec from JSON byte array.
// This should not be needed, but runtime.DefaultUnstructuredConverter.FromUnstructured does not work well with embedded structs.
func (s *RateLimitPolicySpec) UnmarshalJSON(j []byte) error {
	targetRef := struct {
		gwapiv1alpha2.LocalPolicyTargetReferenceWithSectionName `json:"targetRef"`
	}{}
	if err := json.Unmarshal(j, &targetRef); err != nil {
		return err
	}
	s.TargetRef = targetRef.LocalPolicyTargetReferenceWithSectionName

	defaults := &struct {
		*MergeableRateLimitPolicySpec `json:"defaults,omitempty"`
	}{}
	if err := json.Unmarshal(j, defaults); err != nil {
		return err
	}
	s.Defaults = defaults.MergeableRateLimitPolicySpec

	overrides := &struct {
		*MergeableRateLimitPolicySpec `json:"overrides,omitempty"`
	}{}
	if err := json.Unmarshal(j, overrides); err != nil {
		return err
	}
	s.Overrides = overrides.MergeableRateLimitPolicySpec

	proper := struct {
		RateLimitPolicySpecProper `json:""`
	}{}
	if err := json.Unmarshal(j, &proper); err != nil {
		return err
	}
	s.RateLimitPolicySpecProper = proper.RateLimitPolicySpecProper

	return nil
}

func (s *RateLimitPolicySpec) Proper() *RateLimitPolicySpecProper {
	if s.Defaults != nil {
		return &s.Defaults.RateLimitPolicySpecProper
	}

	if s.Overrides != nil {
		return &s.Overrides.RateLimitPolicySpecProper
	}

	return &s.RateLimitPolicySpecProper
}

type MergeableRateLimitPolicySpec struct {
	// Strategy defines the merge strategy to apply when merging this policy with other policies.
	// +kubebuilder:validation:Enum=atomic;merge
	// +kubebuilder:default=atomic
	Strategy string `json:"strategy,omitempty"`

	RateLimitPolicySpecProper `json:""`
}

// RateLimitPolicySpecProper contains common shared fields for defaults and overrides
type RateLimitPolicySpecProper struct {
	// Limits holds the struct of limits indexed by a unique name
	// +optional
	// +kubebuilder:validation:MaxProperties=14
	Limits map[string]Limit `json:"limits,omitempty"`
}

// Limit represents a complete rate limit configuration
type Limit struct {
	// When holds the list of conditions for the policy to be enforced.
	// Called also "soft" conditions as route selectors must also match
	// +optional
	When []WhenCondition `json:"when,omitempty"`

	// Counters defines additional rate limit counters based on context qualifiers and well known selectors
	// TODO Document properly "Well-known selector" https://github.com/Kuadrant/architecture/blob/main/rfcs/0001-rlp-v2.md#well-known-selectors
	// +optional
	Counters []ContextSelector `json:"counters,omitempty"`

	// Rates holds the list of limit rates
	// +optional
	Rates []Rate `json:"rates,omitempty"`
}

// +kubebuilder:validation:Enum:=second;minute;hour;day
type TimeUnit string

// Rate defines the actual rate limit that will be used when there is a match
type Rate struct {
	// Limit defines the max value allowed for a given period of time
	Limit int `json:"limit"`

	// Duration defines the time period for which the Limit specified above applies.
	Duration int `json:"duration"`

	// Duration defines the time uni
	// Possible values are: "second", "minute", "hour", "day"
	Unit TimeUnit `json:"unit"`
}

// WhenCondition defines semantics for matching an HTTP request based on conditions
// https://gateway-api.sigs.k8s.io/reference/spec/#gateway.networking.k8s.io/v1.HTTPRouteSpec
type WhenCondition struct {
	// Selector defines one item from the well known selectors
	// TODO Document properly "Well-known selector" https://github.com/Kuadrant/architecture/blob/main/rfcs/0001-rlp-v2.md#well-known-selectors
	Selector ContextSelector `json:"selector"`

	// The binary operator to be applied to the content fetched from the selector
	// Possible values are: "eq" (equal to), "neq" (not equal to)
	Operator WhenConditionOperator `json:"operator"`

	// The value of reference for the comparison.
	Value string `json:"value"`
}

// ContextSelector defines one item from the well known attributes
// Attributes: https://www.envoyproxy.io/docs/envoy/latest/intro/arch_overview/advanced/attributes
// Well-known selectors: https://github.com/Kuadrant/architecture/blob/main/rfcs/0001-rlp-v2.md#well-known-selectors
// They are named by a dot-separated path (e.g. request.path)
// Example: "request.path" -> The path portion of the URL
// +kubebuilder:validation:MinLength=1
// +kubebuilder:validation:MaxLength=253
type ContextSelector string

// +kubebuilder:validation:Enum:=eq;neq;startswith;endswith;incl;excl;matches
type WhenConditionOperator string

type RateLimitPolicyStatus struct {
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

func (s *RateLimitPolicyStatus) GetConditions() []metav1.Condition {
	return s.Conditions
}

//+kubebuilder:object:root=true

// RateLimitPolicyList contains a list of RateLimitPolicy
type RateLimitPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RateLimitPolicy `json:"items"`
}

func init() {
	SchemeBuilder.Register(&RateLimitPolicy{}, &RateLimitPolicyList{})
}
