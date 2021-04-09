local_resource(
  'generate',
  cmd='make generate',
  deps=['./pkg/apis/core/v1alpha1/manifest_types.go'],
  trigger_mode=TRIGGER_MODE_MANUAL,
  auto_init=False)

local_resource(
  'apiserver',
  serve_cmd='make run-apiserver',
  readiness_probe=probe(http_get=http_get_action(port=9443, path='/readyz')))

local_resource(
  'kubectl-get',
  cmd='kubectl --kubeconfig kubeconfig get manifests',
  trigger_mode=TRIGGER_MODE_MANUAL,
  auto_init=False)

local_resource(
  'kubectl-apply',
  cmd='kubectl --kubeconfig kubeconfig apply -f manifest.yaml',
  trigger_mode=TRIGGER_MODE_MANUAL,
  auto_init=False)
