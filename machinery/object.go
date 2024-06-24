package machinery

import (
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/runtime/schema"
	k8stypes "k8s.io/apimachinery/pkg/types"
)

type Object interface {
	schema.ObjectKind

	GetNamespace() string
	GetName() string
	GetURL() string
}

func namespacedName(namespace, name string) string {
	return k8stypes.NamespacedName{Namespace: namespace, Name: name}.String()
}

func UrlFromObject(obj Object) string {
	name := strings.TrimPrefix(namespacedName(obj.GetNamespace(), obj.GetName()), "/")
	return fmt.Sprintf("%s#%s", strings.ToLower(obj.GroupVersionKind().GroupKind().String()), name)
}
