// Copyright 2025 Cosmos Nicolaou. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"net/url"
	"slices"
	"strings"

	"cloudeng.io/datetime"
	"github.com/cosnicolaou/automation/devices"
	"github.com/cosnicolaou/automation/scheduler"
	"github.com/jedib0t/go-pretty/v6/table"
)

func newCalendarTable(cal *scheduler.Calendar, dr datetime.CalendarDateRange) table.Writer {
	tw := table.NewWriter()
	tw.SetColumnConfigs([]table.ColumnConfig{
		{Number: 1, AutoMerge: true},
		{Number: 2, AutoMerge: true},
	})
	tw.AppendHeader(table.Row{"Date", "Time", "Schedule", "Device", "Operation", "Condition"})
	for day := range dr.Dates() {
		actions := cal.Scheduled(day)
		for _, a := range actions {
			op := a.T.Name
			if len(a.T.Args) > 0 {
				op += "(" + strings.Join(a.T.Args, ", ") + ")"
			}
			pre := ""
			if a.T.Precondition.Condition != nil {
				pre = fmt.Sprintf("if %v.%v", a.T.Precondition.Device, a.T.Precondition.Name)
				if a.T.Precondition.Args != nil {
					pre += "(" + strings.Join(a.T.Precondition.Args, ", ") + ")"
				}
			}
			tod := datetime.NewTimeOfDay(a.When.Hour(), a.When.Minute(), a.When.Second())
			tw.AppendRow(table.Row{day, tod, a.Schedule, a.T.DeviceName, op, pre})
		}
		tw.AppendSeparator()
	}
	return tw
}

func urlForOp(host, device, op string, args []string, cond bool) string {
	params := url.Values{}
	params.Add("device", device)
	params.Add("op", op)
	for _, a := range args {
		params.Add("arg", a)
	}
	if cond {
		return fmt.Sprintf("<a href=\"http://%v/api/condition?%v\">click</a>", host, params.Encode())
	}
	return fmt.Sprintf("<a href=\"http://%v/api/operation?%v\">click</a>", host, params.Encode())
}

func deviceID(dev string, cond, id bool) string {
	if !id {
		return dev
	}
	if cond {
		return fmt.Sprintf("%v<div><id=\"cond:%v\"><div>", dev, dev)
	}
	return fmt.Sprintf("<a id=\"%v\">%v</a>", dev, dev)
}

func operationsRows(device string, ops []string, args map[string][]string, opsHelp map[string]string, host string, condition bool) []table.Row {
	html := len(host) > 0
	rows := []table.Row{}
	for _, op := range ops {
		pars, configured := args[op]
		help := opsHelp[op]
		row := table.Row{
			deviceID(device, condition, html),
			op,
			strings.Join(pars, ", "),
			help,
			configured,
		}
		if configured && html {
			row = append(row, urlForOp(host, device, op, pars, condition))
		}
		rows = append(rows, row)
	}
	return rows
}

func controllerTable(sys devices.System, host string) table.Writer {
	controllerOps := table.NewWriter()
	controllerOps.SetColumnConfigs([]table.ColumnConfig{
		{Number: 1, AutoMerge: true},
	})
	controllerOps.SetTitle("Controller Operations")
	header := table.Row{"Controller", "Operation", "Args", "Help", "Configured"}
	if len(host) > 0 {
		header = append(header, "Click to run")
	}
	controllerOps.AppendHeader(header)
	for _, c := range sys.Config.Controllers {
		rows := operationsRows(
			c.Name,
			opNames(sys.Controllers[c.Name].Operations()),
			c.Operations,
			sys.Controllers[c.Name].OperationsHelp(),
			host,
			false,
		)
		for _, row := range rows {
			controllerOps.AppendRow(row)
		}
	}
	return controllerOps
}

func devicesTable(sys devices.System, conditions bool, host string) table.Writer {
	deviceOps := table.NewWriter()
	deviceOps.SetTitle("Device Operations")
	header := table.Row{"Device", "Operation", "Args", "Help", "Configured"}
	if conditions {
		deviceOps.SetTitle("Device Conditions")
		header[1] = "Condition"
	}
	deviceOps.SetColumnConfigs([]table.ColumnConfig{
		{Number: 1, AutoMerge: true},
	})
	if len(host) > 0 {
		header = append(header, "Click to run")
	}
	deviceOps.AppendHeader(header)

	for _, d := range sys.Config.Devices {
		var rows []table.Row
		if conditions {
			rows = operationsRows(
				d.Name,
				opNames(sys.Devices[d.Name].Conditions()),
				d.Conditions,
				sys.Devices[d.Name].ConditionsHelp(),
				host,
				true)
		} else {
			rows = operationsRows(
				d.Name,
				opNames(sys.Devices[d.Name].Operations()),
				d.Operations,
				sys.Devices[d.Name].OperationsHelp(),
				host,
				false)
		}
		for _, row := range rows {
			deviceOps.AppendRow(row)
		}
	}
	return deviceOps
}

func newOperationsTables(sys devices.System, host string) (controllerOps, deviceOps, deviceConds table.Writer) {
	controllerOps = controllerTable(sys, host)
	deviceOps = devicesTable(sys, false, host)
	deviceConds = devicesTable(sys, true, host)
	return
}

func newDeviceTables(title, header string, devs []string, anchor string, conditions bool) table.Writer {
	tw := table.NewWriter()
	tw.SetTitle(title)
	row := table.Row{header}
	if len(anchor) > 0 {
		row = append(row, "")
	}
	tw.AppendHeader(row)
	for _, d := range devs {
		row := table.Row{d}
		if len(anchor) == 0 {
			row = append(row, d)
			tw.AppendRow(row)
		}
		dn := d
		if conditions {
			dn = "cond:" + d
		}
		row = append(row, fmt.Sprintf("<a href=\"%v#%v\">%v</a>", anchor, dn, d))
		tw.AppendRow(row)
	}
	return tw
}

func newDevicesTables(sys devices.System) (controllers, devices, devicesWithConditions table.Writer) {
	controllers = newDeviceTables("Controllers", "Controller", opNames(sys.Controllers), "/controllers", false)

	devices = newDeviceTables("Devices", "Device", opNames(sys.Devices), "/devices", false)

	hasConditions := []string{}
	for _, d := range sys.Config.Devices {
		if d.Conditions != nil {
			hasConditions = append(hasConditions, d.Name)
		}
	}
	slices.Sort(hasConditions)
	devicesWithConditions = newDeviceTables("Devices with Conditions", "Device", hasConditions, "/conditions", true)

	return
}
