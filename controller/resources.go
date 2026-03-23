package controller

import (
	core "k8s.io/api/core/v1"
	gwapiv1 "sigs.k8s.io/gateway-api/apis/v1"
)

// API Resources
var (
	// core
	ServicesResource   = core.SchemeGroupVersion.WithResource("services")
	ConfigMapsResource = core.SchemeGroupVersion.WithResource("configmaps")

	// gateway api
	GatewayClassesResource = gwapiv1.SchemeGroupVersion.WithResource("gatewayclasses")
	GatewaysResource       = gwapiv1.SchemeGroupVersion.WithResource("gateways")
	GRPCRoutesResource     = gwapiv1.SchemeGroupVersion.WithResource("grpcroutes")
	HTTPRoutesResource     = gwapiv1.SchemeGroupVersion.WithResource("httproutes")
)
