// Copyright 2024 Cosmos Nicolaou. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package scheduler

import (
	"fmt"
	"slices"

	"cloudeng.io/datetime/schedule"
	"github.com/cosnicolaou/automation/devices"
	"gopkg.in/yaml.v3"
)

type timeOfDay string

func (t *timeOfDay) UnmarshalYAML(node *yaml.Node) error {
	var atl ActionTimeList
	*t = timeOfDay(node.Value)
	return atl.Parse(node.Value)
}

// Action represents a single action to be taken on any given day.
type Action struct {
	devices.Action
}

// orderActionsStatic orders the actions in the supplied slice of
// actions according to the before and after constraints in actionDetailed.
func orderActionsStatic(actions schedule.ActionSpecs[Action], detailed []actionDetailed) (schedule.ActionSpecs[Action], error) {
	if len(detailed) == 0 {
		return actions, nil
	}

	order := map[string]int{}
	for i, a := range actions {
		order[a.Name] = i
	}

	for _, wa := range detailed {
		if len(wa.Before) == 0 && len(wa.After) == 0 {
			continue
		}
		before, target, err := validateOpName(wa)
		if err != nil {
			return nil, err
		}

		targetPos, ok := order[target]
		if !ok {
			return nil, fmt.Errorf("action %v not found", target)
		}
		cPos := order[wa.Action]

		if actions[targetPos].Due != actions[cPos].Due {
			return nil, fmt.Errorf("action %v is not scheduled for the same time as %v", target, wa.Action)
		}

		if before {
			if cPos != targetPos-1 {
				tmp := slices.Insert(actions, targetPos, actions[cPos])
				delPos := posPostInsertion(cPos, targetPos)
				return slices.Delete(tmp, delPos, delPos+1), nil
			}
			continue
		}
		if cPos != targetPos+1 {
			tmp := slices.Insert(actions, targetPos+1, actions[cPos])
			delPos := posPostInsertion(cPos, targetPos)
			return slices.Delete(tmp, delPos, delPos+1), nil
		}
	}
	return actions, nil
}

func validateOpName(detailed actionDetailed) (before bool, name string, err error) {
	if len(detailed.Before) != 0 && len(detailed.After) != 0 {
		return false, "", fmt.Errorf("action %v cannot have both before and after specified", detailed.Action)
	}
	name = detailed.Before
	before = true
	if len(name) == 0 {
		name = detailed.After
		before = false
	}
	if name == detailed.Action {
		return false, "", fmt.Errorf("action %v cannot be before or after itself", detailed.Action)
	}
	return

}

func posPostInsertion(cPos, targetPos int) int {
	if cPos >= targetPos {
		return cPos + 1
	}
	return cPos
}
