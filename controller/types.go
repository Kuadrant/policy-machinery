package controller

import gwapiv1 "sigs.k8s.io/gateway-api/apis/v1"

var (
	GatewayClassKind = gwapiv1.SchemeGroupVersion.WithKind("GatewayClass").GroupKind()
	GatewayKind      = gwapiv1.SchemeGroupVersion.WithKind("Gateway").GroupKind()
	HTTPRouteKind    = gwapiv1.SchemeGroupVersion.WithKind("HTTPRoute").GroupKind()

	GatewayClassesResource = gwapiv1.SchemeGroupVersion.WithResource("gatewayclasses")
	GatewaysResource       = gwapiv1.SchemeGroupVersion.WithResource("gateways")
	HTTPRoutesResource     = gwapiv1.SchemeGroupVersion.WithResource("httproutes")
)
