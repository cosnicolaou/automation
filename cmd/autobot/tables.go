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

type tableManager struct {
	html bool
	js   bool
}

func (tm tableManager) Calendar(cal *scheduler.Calendar, dr datetime.CalendarDateRange) table.Writer {
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

func (tm tableManager) RenderHTML(tw table.Writer) string {
	tw.SetStyle(table.Style{
		HTML: table.HTMLOptions{
			CSSClass:    "table",
			EmptyColumn: "&nbsp;",
			EscapeText:  false,
			Newline:     "<br/>",
		}})
	return tw.RenderHTML()
}

func (tm tableManager) withAPICall(device, op string, args []string, configured, cond bool) string {
	if !tm.html || !configured {
		return op
	}
	if tm.js {
		argStr := "[]"
		if len(args) > 0 {
			argStr = fmt.Sprintf("['%v']", strings.Join(args, "', '"))
		}
		jsop := "runOperation"
		if cond {
			jsop = "runCondition"
		}
		return fmt.Sprintf("<button id=\"%v.%v\" onclick=\"%s('%v', '%v', %s)\">%v</button>", device, op, jsop, device, op, argStr, op)
	}

	params := url.Values{}
	params.Add("dev", device)
	params.Add("op", op)
	for _, a := range args {
		params.Add("arg", a)
	}
	if cond {
		return fmt.Sprintf("<a href=\"/api/condition?%v\">%v</a>", params.Encode(), op)
	}
	return fmt.Sprintf("<a href=\"/api/operation?%v\">%v</a>", params.Encode(), op)
}

func (tm tableManager) withDivID(dev string, cond bool) string {
	if !tm.html {
		return dev
	}
	if cond {
		return fmt.Sprintf("%v<div><id=\"cond:%v\"><div>", dev, dev)
	}
	return fmt.Sprintf("<a id=\"%v\">%v</a>", dev, dev)
}

func (tm tableManager) operationsRows(device string, ops []string, args map[string][]string, opsHelp map[string]string, condition bool) []table.Row {
	rows := []table.Row{}
	for _, op := range ops {
		pars, configured := args[op]
		help := opsHelp[op]
		row := table.Row{
			tm.withDivID(device, condition),
			tm.withAPICall(device, op, pars, configured, condition),
			strings.Join(pars, ", "),
			help,
			configured,
		}
		rows = append(rows, row)
	}
	return rows
}

func (tm tableManager) newOperationsTableHeader(tile, optype string) table.Writer {
	tw := table.NewWriter()
	tw.SetTitle(tile)
	tw.SetColumnConfigs([]table.ColumnConfig{
		{Number: 1, AutoMerge: true},
	})
	tw.AppendHeader(table.Row{"Name", optype, "Args", "Help", "Configured"})
	return tw
}

func (tm tableManager) ControllerOperations(sys devices.System) table.Writer {
	tw := tm.newOperationsTableHeader("Controller Operations", "Operation")
	for _, c := range sys.Config.Controllers {
		rows := tm.operationsRows(
			c.Name,
			opNames(sys.Controllers[c.Name].Operations()),
			c.Operations,
			sys.Controllers[c.Name].OperationsHelp(),
			false,
		)
		for _, row := range rows {
			tw.AppendRow(row)
		}
	}
	return tw
}

func (tm tableManager) devicesOrConditions(sys devices.System, conditions bool) []table.Row {
	rows := []table.Row{}
	if conditions {
		for _, d := range sys.Config.Devices {
			nr := tm.operationsRows(
				d.Name,
				opNames(sys.Devices[d.Name].Conditions()),
				d.Conditions,
				sys.Devices[d.Name].ConditionsHelp(),
				true)
			rows = append(rows, nr...)
		}
		return rows
	}
	for _, d := range sys.Config.Devices {
		fmt.Printf("ADDING %v\n", d.Name)
		nr := tm.operationsRows(
			d.Name,
			opNames(sys.Devices[d.Name].Operations()),
			d.Operations,
			sys.Devices[d.Name].OperationsHelp(),
			false)
		rows = append(rows, nr...)
	}
	return rows
}

func (tm tableManager) DeviceOperations(sys devices.System) table.Writer {
	tw := tm.newOperationsTableHeader("Device Operations", "Operation")
	rows := tm.devicesOrConditions(sys, false)
	for _, row := range rows {
		tw.AppendRow(row)
	}
	return tw
}

func (tm tableManager) DeviceConditions(sys devices.System) table.Writer {
	tw := tm.newOperationsTableHeader("Device Conditions", "Conditions")
	rows := tm.devicesOrConditions(sys, true)
	for _, row := range rows {
		tw.AppendRow(row)
	}
	return tw
}

func (tm tableManager) withAnchor(dev, tag string, conditions bool) string {
	if !tm.html {
		return dev
	}
	dn := dev
	if conditions {
		dn = "cond:" + dev
	}
	return fmt.Sprintf("<a href=\"%v#%v\">%v</a>", tag, dn, dev)
}

func (tm tableManager) newList(title string, devs []string, anchor string, conditions bool) table.Writer {
	tw := table.NewWriter()
	tw.SetTitle(title)
	tw.AppendHeader(table.Row{"Name"})
	for _, d := range devs {

		row := table.Row{tm.withAnchor(d, anchor, conditions)}
		tw.AppendRow(row)
	}
	return tw
}

func (tm tableManager) Controllers(sys devices.System) table.Writer {
	devs := opNames(sys.Controllers)
	return tm.newList("Controllers", devs, "/controllers", false)
}

func (tm tableManager) Devices(sys devices.System) table.Writer {
	devs := opNames(sys.Devices)
	return tm.newList("Devices", devs, "/devices", false)
}

func (tm tableManager) Conditions(sys devices.System) table.Writer {
	devs := []string{}
	for _, d := range sys.Config.Devices {
		if d.Conditions != nil {
			devs = append(devs, d.Name)
		}
	}
	slices.Sort(devs)
	return tm.newList("Conditions", devs, "/conditions", false)
}
