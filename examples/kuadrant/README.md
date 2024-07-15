# Kuadrant Controller

Practical example of using the [Policy Machinery](https://github.com/kuadrant/policy-machinery) to implement a custom controller.

<br/>

The example defines 4 kinds of policies:
- **DNSPolicy:** can target Gateways and Listeners
- **TLSPolicy:** can target Gateways and Listeners
- **AuthPolicy:** can target Gateways, Listeners, HTTPRoutes, and HTTPRouteRules; support for Defaults & Overrides and 2 merge strategies (`atomic` or `merge`)
- **RateLimitPolicy:** can target Gateways, Listeners, HTTPRoutes, and HTTPRouteRules; support for Defaults & Overrides and 2 merge strategies (`atomic` or `merge`)

The controller watches for events related to these resources, plus Gateways and HTTPRoutes: It keeps an in-memory Gateway API topology up to date.

A callback to a reconcile function computes the effective policies for every path between Gateways and Listeners (DNSPolicy and TLSPolicy) and between Gateways and HTTPRouteRules (AuthPolicy and RateLimitPolicy), applying the proper merge strategy specified in the policies.

## Demo

### Requirements

- [kubectl](https://kubernetes.io/docs/reference/kubectl/introduction/)
- [Kind](https://kind.sigs.k8s.io/)

### Setup

Create the cluster:

```sh
kind create cluster
```

Install the CRDs:

```sh
make install
```

Run the controller (holds the shell):

```sh
make run
```

### Create the resources

> **Note:** After each step below, check out the state of the topology (`topology.dot`) and the controller logs for the new effective policies in place.

1. Create a Gateway:

```sh
kubectl apply -f -<<EOF
apiVersion: gateway.networking.k8s.io/v1
kind: Gateway
metadata:
  name: prod-web
spec:
  gatewayClassName: example
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
  defaults:
    rules:
      authorization:
        "from8am-to-5pm":
          opa:
            rego: |
              allow { [h, _, _] := time.clock(time.now_ns()); h >= 8; h <= 17 }
EOF
```

5. Create a HTTPRoute-wide AuthPolicy to enforce API key authentication and affiliation to the 'admin' group:

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

6. Add another HTTPRouteRule:

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
  - matches: # rule-1
    - method: POST
    backendRefs:
    - name: my-app
      port: 80
  - matches: # rule-2
    - method: GET
    backendRefs:
    - name: my-app
      port: 80
EOF
```

7. Change the `api-key-admins` AuthPolicy to target one of the HTTPRouteRules only:

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
    sectionName: rule-1 # relies on the order of the HTTPRouteRules (from 1)
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

8. Change the `business-hours` AuthPolicy to an override:

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
EOF
```

9. Change the `business-hours` AuthPolicy to the 'merge' strategy:

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

10. Create a HTTPRoute-wide RateLimitPolicy:

```sh
kubectl apply -f - <<EOF
apiVersion: kuadrant.io/v1beta3
kind: RateLimitPolicy
metadata:
  name: my-app-rl
spec:
  targetRef:
    group: gateway.networking.k8s.io
    kind: HTTPRoute
    name: my-app
  limits:
    "restrictive":
      rates:
      - limit: 5
        duration: 10
        unit: second
EOF
```

11. Define a specific Gateway listener to enable TLS:

```sh
kubectl apply -f -<<EOF
apiVersion: gateway.networking.k8s.io/v1
kind: Gateway
metadata:
  name: prod-web
spec:
  gatewayClassName: example
  listeners:
  - name: http
    protocol: HTTP
    port: 80
    allowedRoutes:
      namespaces:
        from: Same
  - name: https
    protocol: HTTPS
    port: 443
    allowedRoutes:
      namespaces:
        from: Same
    tls:
      mode: Terminate
      certificateRefs:
        - name: gw-tls
          kind: Secret
EOF
```

12. Create a TLSPolicy:

```sh
kubectl apply -f - <<EOF
apiVersion: kuadrant.io/v1alpha2
kind: TLSPolicy
metadata:
  name: https
spec:
  targetRef:
    group: gateway.networking.k8s.io
    kind: Gateway
    name: prod-web
    sectionName: https
  issuerRef:
    name: selfsigned-issuer
EOF
```

### Cleanup

Delete the resources:

```sh
kubectl get gateways,httproutes,dnspolicies,tlspolicies,authpolicies,ratelimitpolicies -o name | while read -r line; do kubectl delete "$line"; done
```

Delete the cluster:

```sh
kind delete cluster
```
