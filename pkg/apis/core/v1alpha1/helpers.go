// Copyright 2020 Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// HasOperation checks if the obj has the given operation annotation
func HasOperation(obj metav1.ObjectMeta, op Operation) bool {
	currentOp, ok := obj.Annotations[OperationAnnotation]
	if !ok {
		return false
	}

	return Operation(currentOp) == op
}

// InitCondition initializes a new Condition with an Unknown status.
func InitCondition(conditionType ConditionType) Condition {
	return Condition{
		Type:               conditionType,
		Status:             ConditionUnknown,
		Reason:             "ConditionInitialized",
		Message:            "The condition has been initialized but its semantic check has not been performed yet.",
		LastTransitionTime: metav1.Now(),
	}
}

// GetCondition returns the condition with the given <conditionType> out of the list of <conditions>.
// In case the required type could not be found, it returns nil.
func GetCondition(conditions []Condition, conditionType ConditionType) *Condition {
	for _, condition := range conditions {
		if condition.Type == conditionType {
			c := condition
			return &c
		}
	}
	return nil
}

// GetOrInitCondition tries to retrieve the condition with the given condition type from the given conditions.
// If the condition could not be found, it returns an initialized condition of the given type.
func GetOrInitCondition(conditions []Condition, conditionType ConditionType) Condition {
	if condition := GetCondition(conditions, conditionType); condition != nil {
		return *condition
	}
	return InitCondition(conditionType)
}

// UpdatedCondition updates the properties of one specific condition.
func UpdatedCondition(condition Condition, status ConditionStatus, reason, message string, codes ...ErrorCode) Condition {
	newCondition := Condition{
		Type:               condition.Type,
		Status:             status,
		Reason:             reason,
		Message:            message,
		LastTransitionTime: condition.LastTransitionTime,
		LastUpdateTime:     metav1.Now(),
		Codes:              codes,
	}

	if condition.Status != status {
		newCondition.LastTransitionTime = metav1.Now()
	}
	return newCondition
}

func CreateOrUpdateConditions(conditions []Condition, condType ConditionType, status ConditionStatus, reason, message string, codes ...ErrorCode) []Condition {
	for i, foundCondition := range conditions {
		if foundCondition.Type == condType {
			conditions[i] = UpdatedCondition(conditions[i], status, reason, message, codes...)
			return conditions
		}
	}

	return append(conditions, UpdatedCondition(InitCondition(condType), status, reason, message, codes...))
}