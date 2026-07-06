/*
 *  Copyright (c) 2026, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 *
 */

package dto

import (
	"fmt"
)

// TimeUnit represents allowed time units for expiration duration
type TimeUnit string

const (
	TimeUnitSeconds TimeUnit = "seconds"
	TimeUnitMinutes TimeUnit = "minutes"
	TimeUnitHours   TimeUnit = "hours"
	TimeUnitDays    TimeUnit = "days"
	TimeUnitWeeks   TimeUnit = "weeks"
	TimeUnitMonths  TimeUnit = "months"
)

// Validate checks that the TimeUnit has one of the allowed values (case-insensitive).
func (t TimeUnit) Validate() error {
	switch t {
	case TimeUnitSeconds, TimeUnitMinutes, TimeUnitHours, TimeUnitDays, TimeUnitWeeks, TimeUnitMonths:
		return nil
	default:
		return fmt.Errorf("unit must be one of [seconds, minutes, hours, days, weeks, months], got %q", t)
	}
}

// ExpirationDuration represents a time duration for API key expiration
type ExpirationDuration struct {
	Duration int      `json:"duration" yaml:"duration"`
	Unit     TimeUnit `json:"unit" yaml:"unit"`
}

// Validate checks that Duration is positive and Unit is valid.
func (e *ExpirationDuration) Validate() error {
	if e.Duration <= 0 {
		return fmt.Errorf("duration must be a positive integer, got %d", e.Duration)
	}

	if err := e.Unit.Validate(); err != nil {
		return err
	}

	return nil
}
