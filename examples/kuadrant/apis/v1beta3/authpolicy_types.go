package v1beta3

import (
	"encoding/json"
	"fmt"
	"strings"

	authorinov1beta2 "github.com/kuadrant/authorino/api/v1beta2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	gwapiv1alpha2 "sigs.k8s.io/gateway-api/apis/v1alpha2"

	"github.com/kuadrant/policy-machinery/machinery"

	kuadrantapis "github.com/kuadrant/policy-machinery/examples/kuadrant/apis"
)

var (
	AuthPolicyKind       = schema.GroupKind{Group: SchemeGroupVersion.Group, Kind: "AuthPolicy"}
	AuthPoliciesResource = SchemeGroupVersion.WithResource("authpolicies")
)

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:metadata:labels="gateway.networking.k8s.io/policy=inherited"
// +kubebuilder:printcolumn:name="Accepted",type=string,JSONPath=`.status.conditions[?(@.type=="Accepted")].status`,description="AuthPolicy Accepted",priority=2
// +kubebuilder:printcolumn:name="Enforced",type=string,JSONPath=`.status.conditions[?(@.type=="Enforced")].status`,description="AuthPolicy Enforced",priority=2
// +kubebuilder:printcolumn:name="TargetKind",type="string",JSONPath=".spec.targetRef.kind",description="Kind of the object to which the policy aaplies",priority=2
// +kubebuilder:printcolumn:name="TargetName",type="string",JSONPath=".spec.targetRef.name",description="Name of the object to which the policy applies",priority=2
// +kubebuilder:printcolumn:name="TargetSection",type="string",JSONPath=".spec.targetRef.sectionName",description="Name of the section within the object to which the policy applies ",priority=2
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// AuthPolicy enables authentication and authorization for service workloads in a Gateway API network
type AuthPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AuthPolicySpec   `json:"spec,omitempty"`
	Status AuthPolicyStatus `json:"status,omitempty"`
}

var _ machinery.Policy = &AuthPolicy{}

func (p *AuthPolicy) GetNamespace() string {
	return p.Namespace
}

func (p *AuthPolicy) GetName() string {
	return p.Name
}

func (p *AuthPolicy) GetIdentity() string {
	return machinery.IdentityFromObject(p)
}

func (p *AuthPolicy) GetTargetRefs() []machinery.PolicyTargetReference {
	return []machinery.PolicyTargetReference{
		machinery.LocalPolicyTargetReferenceWithSectionName{
			LocalPolicyTargetReferenceWithSectionName: p.Spec.TargetRef,
			PolicyNamespace: p.Namespace,
		},
	}
}

func (p *AuthPolicy) GetMergeStrategy() machinery.MergeStrategy {
	if spec := p.Spec.Defaults; spec != nil {
		return kuadrantapis.DefaultsMergeStrategy(spec.Strategy)
	}
	if spec := p.Spec.Overrides; spec != nil {
		return kuadrantapis.OverridesMergeStrategy(spec.Strategy)
	}
	return kuadrantapis.AtomicDefaultsMergeStrategy
}

func (p *AuthPolicy) Merge(other machinery.Policy) machinery.Policy {
	source, ok := other.(*AuthPolicy)
	if !ok {
		return p
	}
	return source.GetMergeStrategy()(source, p)
}

var _ kuadrantapis.MergeablePolicy = &AuthPolicy{}

func (p *AuthPolicy) Empty() bool {
	return p.Spec.Proper().AuthScheme == nil
}

func (p *AuthPolicy) Rules() map[string]any {
	rules := make(map[string]any)

	for ruleId := range p.Spec.Proper().NamedPatterns {
		rules[fmt.Sprintf("patterns#%s", ruleId)] = p.Spec.Proper().NamedPatterns[ruleId]
	}

	for ruleId := range p.Spec.Proper().Conditions {
		rules[fmt.Sprintf("conditions#%d", ruleId)] = p.Spec.Proper().Conditions[ruleId]
	}

	if p.Spec.Proper().AuthScheme == nil {
		return rules
	}

	for ruleId := range p.Spec.Proper().AuthScheme.Authentication {
		rules[fmt.Sprintf("authentication#%s", ruleId)] = p.Spec.Proper().AuthScheme.Authentication[ruleId]
	}

	for ruleId := range p.Spec.Proper().AuthScheme.Metadata {
		rules[fmt.Sprintf("metadata#%s", ruleId)] = p.Spec.Proper().AuthScheme.Metadata[ruleId]
	}

	for ruleId := range p.Spec.Proper().AuthScheme.Authorization {
		rules[fmt.Sprintf("authorization#%s", ruleId)] = p.Spec.Proper().AuthScheme.Authorization[ruleId]
	}

	for ruleId := range p.Spec.Proper().AuthScheme.Callbacks {
		rules[fmt.Sprintf("callbacks#%s", ruleId)] = p.Spec.Proper().AuthScheme.Callbacks[ruleId]
	}

	if p.Spec.Proper().AuthScheme.Response == nil {
		return rules
	}

	rules[fmt.Sprintf("response.unauthenticated#")] = p.Spec.Proper().AuthScheme.Response.Unauthenticated
	rules[fmt.Sprintf("response.unauthorized#")] = p.Spec.Proper().AuthScheme.Response.Unauthorized

	for ruleId := range p.Spec.Proper().AuthScheme.Response.Success.Headers {
		rules[fmt.Sprintf("response.success.headers#%s", ruleId)] = p.Spec.Proper().AuthScheme.Response.Success.Headers[ruleId]
	}

	for ruleId := range p.Spec.Proper().AuthScheme.Response.Success.DynamicMetadata {
		rules[fmt.Sprintf("response.success.metadata#%s", ruleId)] = p.Spec.Proper().AuthScheme.Response.Success.DynamicMetadata[ruleId]
	}

	return rules
}

func (p *AuthPolicy) SetRules(rules map[string]any) {
	ensureNamedPatterns := func() {
		if p.Spec.Proper().NamedPatterns == nil {
			p.Spec.Proper().NamedPatterns = make(map[string]authorinov1beta2.PatternExpressions)
		}
	}

	ensureAuthScheme := func() {
		if p.Spec.Proper().AuthScheme == nil {
			p.Spec.Proper().AuthScheme = &AuthSchemeSpec{}
		}
	}

	ensureAuthentication := func() {
		ensureAuthScheme()
		if p.Spec.Proper().AuthScheme.Authentication == nil {
			p.Spec.Proper().AuthScheme.Authentication = make(map[string]authorinov1beta2.AuthenticationSpec)
		}
	}

	ensureMetadata := func() {
		ensureAuthScheme()
		if p.Spec.Proper().AuthScheme.Metadata == nil {
			p.Spec.Proper().AuthScheme.Metadata = make(map[string]authorinov1beta2.MetadataSpec)
		}
	}

	ensureAuthorization := func() {
		ensureAuthScheme()
		if p.Spec.Proper().AuthScheme.Authorization == nil {
			p.Spec.Proper().AuthScheme.Authorization = make(map[string]authorinov1beta2.AuthorizationSpec)
		}
	}

	ensureResponse := func() {
		ensureAuthScheme()
		if p.Spec.Proper().AuthScheme.Response == nil {
			p.Spec.Proper().AuthScheme.Response = &authorinov1beta2.ResponseSpec{}
		}
	}

	ensureResponseSuccessHeaders := func() {
		ensureResponse()
		if p.Spec.Proper().AuthScheme.Response.Success.Headers == nil {
			p.Spec.Proper().AuthScheme.Response.Success.Headers = make(map[string]authorinov1beta2.HeaderSuccessResponseSpec)
		}
	}

	ensureResponseSuccessDynamicMetadata := func() {
		ensureResponse()
		if p.Spec.Proper().AuthScheme.Response.Success.DynamicMetadata == nil {
			p.Spec.Proper().AuthScheme.Response.Success.DynamicMetadata = make(map[string]authorinov1beta2.SuccessResponseSpec)
		}
	}

	ensureCallbacks := func() {
		ensureAuthScheme()
		if p.Spec.Proper().AuthScheme.Callbacks == nil {
			p.Spec.Proper().AuthScheme.Callbacks = make(map[string]authorinov1beta2.CallbackSpec)
		}
	}

	for id := range rules {
		rule := rules[id]
		parts := strings.SplitN(id, "#", 2)
		group := parts[0]
		ruleId := parts[len(parts)-1]

		if strings.HasPrefix(group, "response.") {
			ensureResponse()
		}

		switch group {
		case "patterns":
			ensureNamedPatterns()
			p.Spec.Proper().NamedPatterns[ruleId] = rule.(authorinov1beta2.PatternExpressions)
		case "conditions":
			p.Spec.Proper().Conditions = append(p.Spec.Proper().Conditions, rule.(authorinov1beta2.PatternExpressionOrRef))
		case "authentication":
			ensureAuthentication()
			p.Spec.Proper().AuthScheme.Authentication[ruleId] = rule.(authorinov1beta2.AuthenticationSpec)
		case "metadata":
			ensureMetadata()
			p.Spec.Proper().AuthScheme.Metadata[ruleId] = rule.(authorinov1beta2.MetadataSpec)
		case "authorization":
			ensureAuthorization()
			p.Spec.Proper().AuthScheme.Authorization[ruleId] = rule.(authorinov1beta2.AuthorizationSpec)
		case "response.unauthenticated":
			ensureResponse()
			p.Spec.Proper().AuthScheme.Response.Unauthenticated = rule.(*authorinov1beta2.DenyWithSpec)
		case "response.unauthorized":
			ensureResponse()
			p.Spec.Proper().AuthScheme.Response.Unauthorized = rule.(*authorinov1beta2.DenyWithSpec)
		case "response.success.headers":
			ensureResponseSuccessHeaders()
			p.Spec.Proper().AuthScheme.Response.Success.Headers[ruleId] = rule.(authorinov1beta2.HeaderSuccessResponseSpec)
		case "response.success.metadata":
			ensureResponseSuccessDynamicMetadata()
			p.Spec.Proper().AuthScheme.Response.Success.DynamicMetadata[ruleId] = rule.(authorinov1beta2.SuccessResponseSpec)
		case "callbacks":
			ensureCallbacks()
			p.Spec.Proper().AuthScheme.Callbacks[ruleId] = rule.(authorinov1beta2.CallbackSpec)
		}
	}
}

// +kubebuilder:validation:XValidation:rule="!(has(self.defaults) && has(self.rules))",message="Implicit and explicit defaults are mutually exclusive"
// +kubebuilder:validation:XValidation:rule="!(has(self.defaults) && has(self.overrides))",message="Overrides and explicit defaults are mutually exclusive"
// +kubebuilder:validation:XValidation:rule="!(has(self.overrides) && has(self.rules))",message="Overrides and implicit defaults are mutually exclusive"
type AuthPolicySpec struct {
	// Reference to the object to which this policy applies.
	// +kubebuilder:validation:XValidation:rule="self.group == 'gateway.networking.k8s.io'",message="Invalid targetRef.group. The only supported value is 'gateway.networking.k8s.io'"
	// +kubebuilder:validation:XValidation:rule="self.kind == 'HTTPRoute' || self.kind == 'Gateway'",message="Invalid targetRef.kind. The only supported values are 'HTTPRoute' and 'Gateway'"
	TargetRef gwapiv1alpha2.LocalPolicyTargetReferenceWithSectionName `json:"targetRef"`

	// Rules to apply as defaults. Can be overridden by more specific policiy rules lower in the hierarchy and by less specific policy overrides.
	// Use one of: defaults, overrides, or bare set of policy rules (implicit defaults).
	// +optional
	Defaults *MergeableAuthPolicySpec `json:"defaults,omitempty"`

	// Rules to apply as overrides. Override all policy rules lower in the hierarchy. Can be overriden by less specific policy overrides.
	// Use one of: defaults, overrides, or bare set of policy rules (implicit defaults).
	// +optional
	Overrides *MergeableAuthPolicySpec `json:"overrides,omitempty"`

	// Bare set of policy rules (implicit defaults).
	// Use one of: defaults, overrides, or bare set of policy rules (implicit defaults).
	AuthPolicySpecProper `json:""`
}

// UnmarshalJSON unmarshals the AuthPolicySpec from JSON byte array.
// This should not be needed, but runtime.DefaultUnstructuredConverter.FromUnstructured does not work well with embedded structs.
func (s *AuthPolicySpec) UnmarshalJSON(j []byte) error {
	targetRef := struct {
		gwapiv1alpha2.LocalPolicyTargetReferenceWithSectionName `json:"targetRef"`
	}{}
	if err := json.Unmarshal(j, &targetRef); err != nil {
		return err
	}
	s.TargetRef = targetRef.LocalPolicyTargetReferenceWithSectionName

	defaults := &struct {
		*MergeableAuthPolicySpec `json:"defaults,omitempty"`
	}{}
	if err := json.Unmarshal(j, defaults); err != nil {
		return err
	}
	s.Defaults = defaults.MergeableAuthPolicySpec

	overrides := &struct {
		*MergeableAuthPolicySpec `json:"overrides,omitempty"`
	}{}
	if err := json.Unmarshal(j, overrides); err != nil {
		return err
	}
	s.Overrides = overrides.MergeableAuthPolicySpec

	proper := struct {
		AuthPolicySpecProper `json:""`
	}{}
	if err := json.Unmarshal(j, &proper); err != nil {
		return err
	}
	s.AuthPolicySpecProper = proper.AuthPolicySpecProper

	return nil
}

func (s *AuthPolicySpec) Proper() *AuthPolicySpecProper {
	if s.Defaults != nil {
		return &s.Defaults.AuthPolicySpecProper
	}

	if s.Overrides != nil {
		return &s.Overrides.AuthPolicySpecProper
	}

	return &s.AuthPolicySpecProper
}

type MergeableAuthPolicySpec struct {
	// Strategy defines the merge strategy to apply when merging this policy with other policies.
	// +kubebuilder:validation:Enum=atomic;merge
	// +kubebuilder:default=atomic
	Strategy string `json:"strategy,omitempty"`

	AuthPolicySpecProper `json:""`
}

// AuthPolicySpecProper contains common shared fields for defaults and overrides
type AuthPolicySpecProper struct {
	// Named sets of patterns that can be referred in `when` conditions and in pattern-matching authorization policy rules.
	// +optional
	NamedPatterns map[string]authorinov1beta2.PatternExpressions `json:"patterns,omitempty"`

	// Overall conditions for the AuthPolicy to be enforced.
	// If omitted, the AuthPolicy will be enforced at all requests to the protected routes.
	// If present, all conditions must match for the AuthPolicy to be enforced; otherwise, the authorization service skips the AuthPolicy and returns to the auth request with status OK.
	// +optional
	Conditions []authorinov1beta2.PatternExpressionOrRef `json:"when,omitempty"`

	// The auth rules of the policy.
	// See Authorino's AuthConfig CRD for more details.
	AuthScheme *AuthSchemeSpec `json:"rules,omitempty"`
}

type AuthSchemeSpec struct {
	// Authentication configs.
	// At least one config MUST evaluate to a valid identity object for the auth request to be successful.
	// +optional
	// +kubebuilder:validation:MaxProperties=10
	Authentication map[string]authorinov1beta2.AuthenticationSpec `json:"authentication,omitempty"`

	// Metadata sources.
	// Authorino fetches auth metadata as JSON from sources specified in this config.
	// +optional
	// +kubebuilder:validation:MaxProperties=10
	Metadata map[string]authorinov1beta2.MetadataSpec `json:"metadata,omitempty"`

	// Authorization policies.
	// All policies MUST evaluate to "allowed = true" for the auth request be successful.
	// +optional
	// +kubebuilder:validation:MaxProperties=10
	Authorization map[string]authorinov1beta2.AuthorizationSpec `json:"authorization,omitempty"`

	// Response items.
	// Authorino builds custom responses to the client of the auth request.
	// +optional
	Response *authorinov1beta2.ResponseSpec `json:"response,omitempty"`

	// Callback functions.
	// Authorino sends callbacks at the end of the auth pipeline to the endpoints specified in this config.
	// +optional
	// +kubebuilder:validation:MaxProperties=10
	Callbacks map[string]authorinov1beta2.CallbackSpec `json:"callbacks,omitempty"`
}

type AuthPolicyStatus struct {
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

func (s *AuthPolicyStatus) GetConditions() []metav1.Condition {
	return s.Conditions
}

//+kubebuilder:object:root=true

// AuthPolicyList contains a list of AuthPolicy
type AuthPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AuthPolicy `json:"items"`
}

func init() {
	SchemeBuilder.Register(&AuthPolicy{}, &AuthPolicyList{})
}
