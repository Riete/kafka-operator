### kubernetes kafka operator

* operator-sdk version: v0.18.1
* kubernetes: v1.14+
* go version: v1.13+
* docker version: 17.03+
* kafka version: 2.12-2.0.0, 2.12-1.1.1


### build 
```
operator-sdk build <IMAGE>:<tag>
```

### deploy operator

```
kubectl apply -f deploy/crds/middleware.io_kafkas_crd.yaml
kubectl apply -f deploy/namespace.yaml
kubectl apply -f deploy/role.yaml
kubectl apply -f deploy/service_account.yaml
kubectl apply -f deploy/role_binding.yaml
kubectl apply -f deploy/operator.yaml # replace image
```

### deploy kafka
```
kubectl apply -f deploy/crds/middleware.io_v1alpha1_kafka_cr.yaml
```