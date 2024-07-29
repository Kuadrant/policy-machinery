package controller

import gwapiv1 "sigs.k8s.io/gateway-api/apis/v1"

var (
	GatewayClassesResource = gwapiv1.SchemeGroupVersion.WithResource("gatewayclasses")
	GatewaysResource       = gwapiv1.SchemeGroupVersion.WithResource("gateways")
	HTTPRoutesResource     = gwapiv1.SchemeGroupVersion.WithResource("httproutes")
)
