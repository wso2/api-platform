# open-choreo-operator Helm chart

This chart packages the Open Choreo Operator for installation via Helm.

Installation examples:

```bash
# install the chart into the cluster (creates namespace by default)
helm install open-choreo-operator helm/open-choreo-operator --create-namespace

# to override image
helm upgrade --install open-choreo-operator helm/open-choreo-operator --set image.repository=myregistry/oc-apipg-controller,image.tag=v0.1.0
```

Notes:
- CRDs are included under `config/crd` and must be installed before resources that depend on them. You can run `make install` to install CRDs or use `kubectl apply -f config/crd` before installing the chart.
