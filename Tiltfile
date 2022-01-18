# -*- mode: Python -*-

# Settings and defaults.

project_name = 'newrelic-k8s-metrics-adapter'

settings = {
  'kind_cluster_name': 'kind',
  'live_reload': True,
  'chart_path': '../helm-charts-newrelic/charts/%s/' % project_name,
}

settings.update(read_json('tilt_option.json', default={}))

default_registry(settings.get('default_registry'))

# Only use explicitly allowed kubeconfigs as a safety measure.
allow_k8s_contexts(settings.get("allowed_contexts", "kind-" + settings.get('kind_cluster_name')))


# Building Docker image.
load('ext://restart_process', 'docker_build_with_restart')

if settings.get('live_reload'):
  binary_name = '%s-linux' % project_name

  # Building daemon binary locally.
  local_resource('%s-binary' % project_name, 'GOOS=linux make build', deps=[
    './main.go',
    './internal',
  ])

  # Use custom Dockerfile for Tilt builds, which only takes locally built daemon binary for live reloading.
  dockerfile = '''
    FROM alpine:3.15
    COPY %s /usr/local/bin/%s
    ENTRYPOINT ["%s"]
  ''' % (binary_name, project_name, project_name)

  docker_build_with_restart(project_name, '.',
    dockerfile_contents=dockerfile,
    entrypoint=[project_name],
    only=binary_name,
    live_update=[
      # Copy the binary so it gets restarted.
      sync(binary_name, '/usr/local/bin/%s' % project_name),
    ],
  )
else:
  docker_build(project_name, '.')

# Deploying Kubernetes resources.
k8s_yaml(helm(settings.get('chart_path'), name=project_name, values=['values-dev.yaml', 'values-local.yaml']))

# Tracking the deployment.
k8s_resource(project_name)
