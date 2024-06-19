package machinery

// Policy contains a PolicySpec that can be merged with another PolicySpec based on a given MergeStrategy.
type Policy interface {
	GetSpec() PolicySpec

	Merge(Policy, MergeStrategy) Policy
}

// PolicySpec contains a list of policy rules.
// It can be merged with another PolicySpec based on a given MergeStrategy.
type PolicySpec interface {
	DeepCopy() PolicySpec
	SetRules([]Rule)
	GetRules() []Rule

	Merge(PolicySpec, MergeStrategy) PolicySpec
}

// Rule represents a policy rule, containing an ID that uniquely identifies the rule within the policy and a spec.
type Rule interface {
	GetId() RuleId
}

type RuleId string

// MergeStrategy is a function that merges two PolicySpecs into a new PolicySpec.
type MergeStrategy func(PolicySpec, PolicySpec) PolicySpec
