/*
Copyright 2021 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package machine

import (
	powervsproviderv1 "github.com/openshift/cluster-api-provider-powervs/pkg/apis/powervsprovider/v1alpha1"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	machinev1 "github.com/openshift/api/machine/v1beta1"
	machinecontroller "github.com/openshift/machine-api-operator/pkg/controller/machine"
	//"k8s.io/klog/v2"
)

const (
	// upstreamMachineClusterIDLabel is the label that a machine must have to identify the cluster to which it belongs
	upstreamMachineClusterIDLabel = "sigs.k8s.io/cluster-api-cluster"
)

// setPowerVSMachineProviderCondition sets the condition for the machine and
// returns the new slice of conditions.
// If the machine does not already have a condition with the specified type,
// a condition will be added to the slice
// If the machine does already have a condition with the specified type,
// the condition will be updated if either of the following are true.
func setPowerVSMachineProviderCondition(condition powervsproviderv1.PowerVSMachineProviderCondition, conditions []powervsproviderv1.PowerVSMachineProviderCondition) []powervsproviderv1.PowerVSMachineProviderCondition {
	now := metav1.Now()

	if existingCondition := findProviderCondition(conditions, condition.Type); existingCondition == nil {
		condition.LastProbeTime = now
		condition.LastTransitionTime = now
		conditions = append(conditions, condition)
	} else {
		updateExistingCondition(&condition, existingCondition)
	}

	return conditions
}

func findProviderCondition(conditions []powervsproviderv1.PowerVSMachineProviderCondition, conditionType powervsproviderv1.PowerVSMachineProviderConditionType) *powervsproviderv1.PowerVSMachineProviderCondition {
	for i := range conditions {
		if conditions[i].Type == conditionType {
			return &conditions[i]
		}
	}
	return nil
}

func updateExistingCondition(newCondition, existingCondition *powervsproviderv1.PowerVSMachineProviderCondition) {
	if !shouldUpdateCondition(newCondition, existingCondition) {
		return
	}

	if existingCondition.Status != newCondition.Status {
		existingCondition.LastTransitionTime = metav1.Now()
	}
	existingCondition.Status = newCondition.Status
	existingCondition.Reason = newCondition.Reason
	existingCondition.Message = newCondition.Message
	existingCondition.LastProbeTime = newCondition.LastProbeTime
}

func shouldUpdateCondition(newCondition, existingCondition *powervsproviderv1.PowerVSMachineProviderCondition) bool {
	return newCondition.Reason != existingCondition.Reason || newCondition.Message != existingCondition.Message
}

func conditionSuccess() powervsproviderv1.PowerVSMachineProviderCondition {
	return powervsproviderv1.PowerVSMachineProviderCondition{
		Type:    powervsproviderv1.MachineCreation,
		Status:  corev1.ConditionTrue,
		Reason:  powervsproviderv1.MachineCreationSucceeded,
		Message: "Machine successfully created",
	}
}

func conditionFailed() powervsproviderv1.PowerVSMachineProviderCondition {
	return powervsproviderv1.PowerVSMachineProviderCondition{
		Type:   powervsproviderv1.MachineCreation,
		Status: corev1.ConditionFalse,
		Reason: powervsproviderv1.MachineCreationFailed,
	}
}

// validateMachine check the label that a machine must have to identify the cluster to which it belongs is present.
func validateMachine(machine machinev1.Machine) error {
	if machine.Labels[machinev1.MachineClusterIDLabel] == "" {
		return machinecontroller.InvalidMachineConfiguration("%v: missing %q label", machine.GetName(), machinev1.MachineClusterIDLabel)
	}

	return nil
}

// getClusterID get cluster ID by machine.openshift.io/cluster-api-cluster label
func getClusterID(machine *machinev1.Machine) (string, bool) {
	clusterID, ok := machine.Labels[machinev1.MachineClusterIDLabel]
	// TODO: remove 347-350
	// NOTE: This block can be removed after the label renaming transition to machine.openshift.io
	if !ok {
		clusterID, ok = machine.Labels[upstreamMachineClusterIDLabel]
	}
	return clusterID, ok
}
