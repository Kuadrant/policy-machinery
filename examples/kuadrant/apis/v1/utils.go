package v1

import (
	"encoding/json"
	"sort"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayapiv1 "sigs.k8s.io/gateway-api/apis/v1"
)

type PolicyStatus interface {
	GetConditions() []metav1.Condition
}

type Policy interface {
	client.Object
	GetTargetRef() gatewayapiv1.LocalPolicyTargetReference
	GetStatus() PolicyStatus
}

func ConditionMarshal(conditions []metav1.Condition) ([]byte, error) {
	conds := append([]metav1.Condition(nil), conditions...)
	sort.Slice(conds, func(i, j int) bool {
		return conds[i].Type < conds[j].Type
	})
	return json.Marshal(conds)
}
