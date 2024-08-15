package controller

import (
	core "k8s.io/api/core/v1"
	gwapiv1 "sigs.k8s.io/gateway-api/apis/v1"
)

// GroupKinds
var (
	// core
	ServiceKind = core.SchemeGroupVersion.WithKind("Service").GroupKind()

	// gateway api
	GatewayClassKind = gwapiv1.SchemeGroupVersion.WithKind("GatewayClass").GroupKind()
	GatewayKind      = gwapiv1.SchemeGroupVersion.WithKind("Gateway").GroupKind()
	HTTPRouteKind    = gwapiv1.SchemeGroupVersion.WithKind("HTTPRoute").GroupKind()
)

// API Resources
var (
	// core
	ServicesResource   = core.SchemeGroupVersion.WithResource("services")
	ConfigMapsResource = core.SchemeGroupVersion.WithResource("configmaps")

	// gateway api
	GatewayClassesResource = gwapiv1.SchemeGroupVersion.WithResource("gatewayclasses")
	GatewaysResource       = gwapiv1.SchemeGroupVersion.WithResource("gateways")
	HTTPRoutesResource     = gwapiv1.SchemeGroupVersion.WithResource("httproutes")
)
