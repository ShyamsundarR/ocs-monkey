"""Helper functions."""

import time

import kubernetes
import kubernetes.client as k8s
from kubernetes.client.rest import ApiException

from pprint import pprint

def handle_api(func, *args, **kwargs):
    """
    Call kubernetes.client APIs and handle Execptions.

    Params:
        func:  The API function
        codes: A dict of how to respond to exeception codes
        (other args are passed to func)

    Returns:
        The return value from calling func

    """
    fkwargs = kwargs.copy()
    codes = kwargs.get("codes")
    if codes is None:
        codes = {500: "retry"}
    else:
        del fkwargs["codes"]
    while True:
        try:
            result = func(*args, **fkwargs)
            break
        except ApiException as ex:
            action = codes.get(ex.status)
            if action == "ignore":
                return result
            if action == "retry":
                time.sleep(1)
                continue
            raise
    return result


def start_and_waitfor_pod(pod_dict):
    """
    Create a pod and wait for it to be Running.

    Params:
        pod_dict: A dict describing the pod to create

    Returns:
        A V1Pod that has achieved the "Running" status.phase

    """
    core_v1 = k8s.CoreV1Api()
    pod = handle_api(core_v1.create_namespaced_pod,
                     namespace=pod_dict["metadata"]["namespace"],
                     body=pod_dict)

    watch = kubernetes.watch.Watch()
    for event in watch.stream(
            func=core_v1.list_namespaced_pod,
            namespace=pod.metadata.namespace,
            field_selector=f"metadata.name={pod.metadata.name}"):
        if event["object"].status.phase == "Running":
            watch.stop()
    return pod

def start_and_waitfor_deployment(deployment_dict):
    """
    Create a deployment and wait for it to reach desired replica count.

    Params:
        deployment_dict: A dict describing the pod to create

    Returns:
        A V1Deployment that has achieved the requested replicas in status.ready_replicas

    """
    apps_v1 = k8s.AppsV1Api()
    deployment = handle_api(apps_v1.create_namespaced_deployment,
                     namespace=deployment_dict["metadata"]["namespace"],
                     body=deployment_dict)

    replica_count=deployment_dict["spec"]["replicas"]
    watch = kubernetes.watch.Watch()
    for event in watch.stream(
            func=apps_v1.list_namespaced_deployment,
            namespace=deployment.metadata.namespace,
            field_selector=f"metadata.name={deployment.metadata.name}"):
        pprint (event)
        if event["object"].status.ready_replicas == deployment.spec.replicas:
            watch.stop()
    return deployment

def start_and_waitfor_job_completion(pod_dict):
    batch_v1 = k8s.BatchV1Api()
    core_v1 = k8s.CoreV1Api()
    pod = handle_api(batch_v1.create_namespaced_job,
                     namespace=pod_dict["metadata"]["namespace"],
                     body=pod_dict)

    watch = kubernetes.watch.Watch()
    for event in watch.stream(
            func=batch_v1.list_namespaced_job,
            namespace=pod.metadata.namespace,
            field_selector=f"metadata.name={pod.metadata.name}"):
        if event["object"].status.succeeded == 1:
            watch.stop()

    handle_api(batch_v1.delete_namespaced_job,
               namespace=pod.metadata.namespace,
               name=pod.metadata.name,
               body=k8s.V1DeleteOptions())

    handle_api(core_v1.delete_collection_namespaced_pod,
               namespace=pod.metadata.namespace)

    return
