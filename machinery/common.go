package machinery

import (
	"fmt"

	"k8s.io/apimachinery/pkg/runtime/schema"
	k8stypes "k8s.io/apimachinery/pkg/types"
)

func namespacedName(namespace, name string) string {
	return k8stypes.NamespacedName{Namespace: namespace, Name: name}.String()
}

func namespacedNameWithSectionName(namespace, name, section string) string {
	return fmt.Sprintf("%s#%s", namespacedName(namespace, name), section)
}

func objectKind(obj schema.ObjectKind) string {
	return obj.GroupVersionKind().GroupKind().String()
}
