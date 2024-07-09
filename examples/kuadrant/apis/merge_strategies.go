package apis

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"

	"github.com/kuadrant/policy-machinery/machinery"
)

const (
	AtomicMergeStrategy     = "atomic"
	PolicyRuleMergeStrategy = "merge"
)

// +kubebuilder:object:generate=false
type MergeablePolicy interface {
	machinery.Policy

	Rules() map[string]any
	SetRules(map[string]any)
	Empty() bool

	DeepCopyObject() runtime.Object
	GetCreationTimestamp() metav1.Time
}

// AtomicDefaultsMergeStrategy implements a merge strategy that returns the target Policy if it exists,
// otherwise it returns the source Policy.
func AtomicDefaultsMergeStrategy(source, target machinery.Policy) machinery.Policy {
	if source == nil {
		return target
	}
	if target == nil {
		return source
	}

	mergeableTargetPolicy := target.(MergeablePolicy)

	if !mergeableTargetPolicy.Empty() {
		return mergeableTargetPolicy.DeepCopyObject().(machinery.Policy)
	}

	return source.(MergeablePolicy).DeepCopyObject().(machinery.Policy)
}

var _ machinery.MergeStrategy = AtomicDefaultsMergeStrategy

// AtomicOverridesMergeStrategy implements a merge strategy that overrides a target Policy with
// a source one.
func AtomicOverridesMergeStrategy(source, _ machinery.Policy) machinery.Policy {
	if source == nil {
		return nil
	}
	return source.(MergeablePolicy).DeepCopyObject().(machinery.Policy)
}

var _ machinery.MergeStrategy = AtomicOverridesMergeStrategy

// PolicyRuleDefaultsMergeStrategy implements a merge strategy that merges a source Policy into a target one
// by keeping the policy rules from the target and adding the ones from the source that do not exist in the target.
func PolicyRuleDefaultsMergeStrategy(source, target machinery.Policy) machinery.Policy {
	if source == nil {
		return target
	}
	if target == nil {
		return source
	}

	sourceMergeablePolicy := source.(MergeablePolicy)
	targetMergeablePolicy := target.(MergeablePolicy)

	// copy rules from the target
	rules := targetMergeablePolicy.Rules()

	// add extra rules from the source
	for ruleId, rule := range sourceMergeablePolicy.Rules() {
		if _, ok := targetMergeablePolicy.Rules()[ruleId]; !ok {
			rules[ruleId] = rule
		}
	}

	mergedPolicy := targetMergeablePolicy.DeepCopyObject().(MergeablePolicy)
	mergedPolicy.SetRules(rules)
	return mergedPolicy
}

var _ machinery.MergeStrategy = PolicyRuleDefaultsMergeStrategy

// PolicyRuleOverridesMergeStrategy implements a merge strategy that merges a source Policy into a target one
// by using the policy rules from the source and keeping from the target only the policy rules that do not exist in
// the source.
func PolicyRuleOverridesMergeStrategy(source, target machinery.Policy) machinery.Policy {
	sourceMergeablePolicy := source.(MergeablePolicy)
	targetMergeablePolicy := target.(MergeablePolicy)

	// copy rules from the source
	rules := sourceMergeablePolicy.Rules()

	// add extra rules from the target
	for ruleId, rule := range targetMergeablePolicy.Rules() {
		if _, ok := sourceMergeablePolicy.Rules()[ruleId]; !ok {
			rules[ruleId] = rule
		}
	}

	mergedPolicy := targetMergeablePolicy.DeepCopyObject().(MergeablePolicy)
	mergedPolicy.SetRules(rules)
	return mergedPolicy
}

var _ machinery.MergeStrategy = PolicyRuleOverridesMergeStrategy

func DefaultsMergeStrategy(strategy string) machinery.MergeStrategy {
	switch strategy {
	case AtomicMergeStrategy:
		return AtomicDefaultsMergeStrategy
	case PolicyRuleMergeStrategy:
		return PolicyRuleDefaultsMergeStrategy
	default:
		return AtomicDefaultsMergeStrategy
	}
}

func OverridesMergeStrategy(strategy string) machinery.MergeStrategy {
	switch strategy {
	case AtomicMergeStrategy:
		return AtomicOverridesMergeStrategy
	case PolicyRuleMergeStrategy:
		return PolicyRuleOverridesMergeStrategy
	default:
		return AtomicOverridesMergeStrategy
	}
}

// +kubebuilder:object:generate=false
type PolicyByCreationTimestamp []MergeablePolicy

func (a PolicyByCreationTimestamp) Len() int      { return len(a) }
func (a PolicyByCreationTimestamp) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a PolicyByCreationTimestamp) Less(i, j int) bool {
	p1Time := ptr.To(a[i].GetCreationTimestamp())
	p2Time := ptr.To(a[j].GetCreationTimestamp())
	if !p1Time.Equal(p2Time) {
		return p1Time.Before(p2Time)
	}

	//  The policy appearing first in alphabetical order by "{namespace}/{name}".
	return fmt.Sprintf("%s/%s", a[i].GetNamespace(), a[i].GetName()) < fmt.Sprintf("%s/%s", a[j].GetNamespace(), a[j].GetName())
}
