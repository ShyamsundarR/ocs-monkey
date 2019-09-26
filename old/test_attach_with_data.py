"""Test fixtures for pytest."""

import random
import time

import kubernetes
import kubernetes.client as k8s
import pytest

import kube

from util import handle_api, start_and_waitfor_job_completion, start_and_waitfor_deployment, start_and_waitfor_pod


files = [1,4,9,15,30,60,120]
dirs = [6]
depths = [5]

@pytest.fixture(params=files)
def filecount(request):
    return request.param

@pytest.fixture(params=dirs)
def dircount(request):
    return request.param

@pytest.fixture(params=depths)
def dirdepth(request):
    return request.param

@pytest.fixture
def filecounts(request, filecount, dircount, dirdepth):
    return [filecount, dircount, dirdepth]

mydeployment = {
    "kind": "Deployment",
    "metadata": {
        "labels": {
            "app": "mypod"
        },
        "name": "mypod-deployment",
    },
    "spec": {
        "replicas": 1,
        "selector": {
            "matchLabels": {
                "app": "mypod"
            }
        },
        "template": {
            "metadata": {
                "labels": {
                    "app": "mypod"
                }
            },
            "spec": {
                "containers": [
                    {
                        "command": [
                            "sleep",
                            "99999"
                        ],
                        "image": "busybox",
                        "imagePullPolicy": "IfNotPresent",
                        "name": "busybox",
                        "volumeMounts": [
                            {
                                "mountPath": "/mnt",
                                "name": "mypvc"
                            }
                        ]
                    }
                ],
                "terminationGracePeriodSeconds": 0,
                "volumes": [
                    {
                        "name": "mypvc",
                        "persistentVolumeClaim": {
                            "claimName": "mypvc"
                        }
                    }
                ]
            }
        }
    }
}

first_iter=True


@pytest.mark.attachwithdata
def test_attach_time(benchmark, unique_namespace, storageclass_iterator, filecounts):
    global first_iter
    core_v1 = k8s.CoreV1Api()
    apps_v1 = k8s.AppsV1Api()

    namespace = unique_namespace

    # Create a PVC
    pvc = handle_api(core_v1.create_namespaced_persistent_volume_claim,
                     namespace=namespace["metadata"]["name"],
                     body={
                         "metadata": {
                             "name": "mypvc",
                             "namespace": namespace["metadata"]["name"]
                         },
                         "spec": {
                             "accessModes": ["ReadWriteOnce"],
                             "resources": {
                                 "requests": {
                                     "storage": "50Gi"
                                 }
                             },
                             "storageClassName": storageclass_iterator
                         }
                     })

    job = {
        "apiVersion": "batch/v1",
        "kind": "Job",
        "metadata": {
            "name": "mypod",
            "namespace": namespace["metadata"]["name"]
        },
        "spec": {
            "template": {
                "spec": {
                    "containers": [{
                        "name": "attach-time-tester",
                        "image": "quay.io/shyamsundarr/filecounts:test",
                        "imagePullPolicy": "IfNotPresent",
                        "args": [
                            "-testdir=/mnt",
                            "-filecount="+str(filecounts[0]),
                            "-dircount="+str(filecounts[1]),
                            "-dirdepth="+str(filecounts[2]),
                            "-tsr=false"
                        ],
                        "volumeMounts": [{
                            "name": "data",
                            "mountPath": "/mnt"
                        }]
                    }],
                    "restartPolicy": "Never",
                    "terminationGracePeriodSeconds": 0,
                    "volumes": [{
                        "name": "data",
                        "persistentVolumeClaim": {
                            "claimName": pvc.metadata.name
                        }
                    }]
                }
            }
        }
    }

    # create pod
    # wait for completion (finish creating data)
    # delete pod
    start_and_waitfor_job_completion(job)

    mydeployment["metadata"]["namespace"] = namespace["metadata"]["name"]
    first_iter=True

    def restart_deployment():
        out_deployment = start_and_waitfor_deployment(mydeployment)

    def setup_wait():
        global first_iter
        # no deployment to delete in the first iteration
        if not first_iter:
            handle_api(apps_v1.delete_namespaced_deployment,
                       namespace=mydeployment["metadata"]["namespace"],
                       name=mydeployment["metadata"]["name"],
                       body=k8s.V1DeleteOptions())
        first_iter=False
        time.sleep(10)
        return

    # benchmark start pod (deletes it as well)
    benchmark.pedantic(restart_deployment, setup=setup_wait, rounds=5)

    # delete last deployment, as setup will not be invoked
    handle_api(apps_v1.delete_namespaced_deployment,
               namespace=mydeployment["metadata"]["namespace"],
               name=mydeployment["metadata"]["name"],
               body=k8s.V1DeleteOptions())

    # delete pvc
    handle_api(core_v1.delete_namespaced_persistent_volume_claim,
               namespace=pvc.metadata.namespace,
               name=pvc.metadata.name,
               body=k8s.V1DeleteOptions())


if __name__ == '__main__':
    pytest.main([__file__])
