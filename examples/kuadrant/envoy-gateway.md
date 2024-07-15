# Kuadrant Controller with Envoy Gateway

The example [custom controller](./README.md) working alongside with [Envoy Gateway](https://gateway.envoyproxy.io/).

This example demonstrates how a controller can use the topology for reconciling other generic objects as well, along with targetables and policies.

<br/>

The controller watches for events related to:
- the 4 kinds of custom policies: DNSPolicy, TLSPolicy, AuthPolicy, and RateLimitPolicy;
- Gateway API resources: GatewayClass, Gateway, and HTTPRoute;
- Envoy Gateway resources: SecurityPolicy.

Apart from computing effective policies, the callback reconcile function also manages Envoy Gateway SecurityPolicy custom resources (create/update/delete) (used internally to implement the AuthPolicies.)

## Demo

### Requirements

- [kubectl](https://kubernetes.io/docs/reference/kubectl/introduction/)
- [Kind](https://kind.sigs.k8s.io/)

### Setup

Create the cluster:

```sh
kind create cluster
```

Install Envoy Gateway:

```sh
make install-envoy-gateway
```

Install the CRDs:

```sh
make install-kuadrant
```

Run the controller (holds the shell):

```sh
make run PROVIDER=envoygateway
```

### Create the resources

> **Note:** After each step below, check out the state of the topology (`topology.dot`).

1. Create a Gateway managed by the Envoy Gateway gateway controller:

```sh
kubectl apply -f -<<EOF
apiVersion: gateway.networking.k8s.io/v1
kind: GatewayClass
metadata:
  name: eg
spec:
  controllerName: gateway.envoyproxy.io/gatewayclass-controller
EOF
```

```sh
kubectl apply -f -<<EOF
apiVersion: gateway.networking.k8s.io/v1
kind: Gateway
metadata:
  name: prod-web
spec:
  gatewayClassName: eg
  listeners:
  - name: http
    protocol: HTTP
    port: 80
    allowedRoutes:
      namespaces:
        from: Same
EOF
```

2. Create a HTTPRoute:

```sh
kubectl apply -f -<<EOF
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: my-app
spec:
  parentRefs:
  - name: prod-web
  hostnames:
  - example.com
  rules:
  - matches:
    - method: POST
    - method: GET
    backendRefs:
    - name: my-app
      port: 80
EOF
```

3. Create a DNSPolicy attached to the Gateway:

```sh
kubectl apply -f - <<EOF
apiVersion: kuadrant.io/v1alpha2
kind: DNSPolicy
metadata:
  name: geo
spec:
  targetRef:
    group: gateway.networking.k8s.io
    kind: Gateway
    name: prod-web
  loadBalancing:
    weighted:
      defaultWeight: 100
    geo:
      defaultGeo: US
  routingStrategy: loadbalanced
EOF
```

4. Create a Gateway-wide AuthPolicy allowing access to the services between 8am to 5pm only by default:

```sh
kubectl apply -f - <<EOF
apiVersion: kuadrant.io/v1beta3
kind: AuthPolicy
metadata:
  name: business-hours
spec:
  targetRef:
    group: gateway.networking.k8s.io
    kind: Gateway
    name: prod-web
  overrides:
    rules:
      authorization:
        "from8am-to-5pm":
          opa:
            rego: |
              allow { [h, _, _] := time.clock(time.now_ns()); h >= 8; h <= 17 }
    strategy: merge
EOF
```

5. Try to delete the Envoy Gateway SecurityPolicy:

```sh
kubectl delete securitypolicy/prod-web
```

6. Create a HTTPRoute-wide AuthPolicy to enforce API key authentication and affiliation to the 'admin' group:

```sh
kubectl apply -f - <<EOF
apiVersion: kuadrant.io/v1beta3
kind: AuthPolicy
metadata:
  name: api-key-admins
spec:
  targetRef:
    group: gateway.networking.k8s.io
    kind: HTTPRoute
    name: my-app
  rules:
    authentication:
      "api-key-authn":
        apiKey:
          selector: {}
        credentials:
          authorizationHeader:
            prefix: APIKEY
    authorization:
      "only-admins":
        opa:
          rego: |
            groups := split(object.get(input.auth.identity.metadata.annotations, "kuadrant.io/groups", ""), ",")
            allow { groups[_] == "admins" }
EOF
```

7. Create another Gateway that is not managed by Envoy Gateway:

```sh
kubectl apply -f -<<EOF
apiVersion: gateway.networking.k8s.io/v1
kind: GatewayClass
metadata:
  name: istio
spec:
  controllerName: istio.io/gateway-controller
EOF
```

```sh
kubectl apply -f -<<EOF
apiVersion: gateway.networking.k8s.io/v1
kind: Gateway
metadata:
  name: other-gateway
spec:
  gatewayClassName: istio
  listeners:
  - name: http
    protocol: HTTP
    port: 80
    allowedRoutes:
      namespaces:
        from: Same
EOF
```

8. Update the HTTPRoute to attach to both gateways:

```sh
kubectl apply -f -<<EOF
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: my-app
spec:
  parentRefs:
  - name: prod-web
  - name: other-gateway
  hostnames:
  - example.com
  rules:
  - matches:
    - method: POST
    - method: GET
    backendRefs:
    - name: my-app
      port: 80
EOF
```

9. Delete the Gateway-wide AuthPolicy:

```sh
kubectl delete authpolicy/business-hours
```

10. Delete the HTTPRoute-wide AuthPolicy:

```sh
kubectl delete authpolicy/api-key-admins
```

### Cleanup

Delete the resources:

```sh
kubectl get gateways,httproutes,dnspolicies,authpolicies,securitypolicies -o name | while read -r line; do kubectl delete "$line"; done
kubectl delete gatewayclass/eg
kubectl delete gatewayclass/istio
```

Delete the cluster:

```sh
kind delete cluster
```
