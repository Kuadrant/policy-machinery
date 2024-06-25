package machinery

import (
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/runtime/schema"
	k8stypes "k8s.io/apimachinery/pkg/types"
	gwapiv1 "sigs.k8s.io/gateway-api/apis/v1"
)

const (
	kindNameURLSeparator        = ':'
	nameSectionNameURLSeparator = '#'
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

func namespacedSectionName(namespace string, sectionName gwapiv1.SectionName) string {
	return fmt.Sprintf("%s%s%s", namespace, string(nameSectionNameURLSeparator), sectionName)
}

func UrlFromObject(obj Object) string {
	name := strings.TrimPrefix(namespacedName(obj.GetNamespace(), obj.GetName()), string(k8stypes.Separator))
	return fmt.Sprintf("%s%s%s", strings.ToLower(obj.GroupVersionKind().GroupKind().String()), string(kindNameURLSeparator), name)
}
