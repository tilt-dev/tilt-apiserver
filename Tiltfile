local_resource(
  'generate',
  cmd='make generate',
  deps=['./pkg/apis/core/v1alpha1/manifest_types.go'])

local_resource(
  'apiserver',
  serve_cmd='make run-apiserver',
  resource_deps=['generate'])

local_resource(
  'kubectl-get',
  cmd='kubectl --kubeconfig kubeconfig --username tilt --password dev get manifests',
  trigger_mode=TRIGGER_MODE_MANUAL,
  auto_init=False)

local_resource(
  'kubectl-apply',
  cmd='kubectl --kubeconfig kubeconfig --username tilt --password dev apply -f manifest.yaml',
  trigger_mode=TRIGGER_MODE_MANUAL,
  auto_init=False)
