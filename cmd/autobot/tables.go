// Copyright 2025 Cosmos Nicolaou. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"cloudeng.io/datetime"
	"github.com/cosnicolaou/automation/cmd/autobot/internal/webapi"
	"github.com/cosnicolaou/automation/devices"
	"github.com/cosnicolaou/automation/internal/logging"
	"github.com/cosnicolaou/automation/scheduler"
	"github.com/jedib0t/go-pretty/v6/table"
)

type tableManager struct {
	html  bool
	jsapi bool
}

func formatOperationWithArgs(a scheduler.Action) string {
	op := a.Name
	if len(a.Args) > 0 {
		op += "(" + strings.Join(a.Args, ", ") + ")"
	}
	return op
}

func formatConditionWithArgs(a scheduler.Action) string {
	pre := ""
	if a.Precondition.Condition != nil {
		pre = fmt.Sprintf("if %v.%v", a.Precondition.Device, a.Precondition.Name)
		if a.Precondition.Args != nil {
			pre += "(" + strings.Join(a.Precondition.Args, ", ") + ")"
		}
	}
	return pre
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
			op := formatOperationWithArgs(a.T)
			pre := formatConditionWithArgs(a.T)
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

func argsAsJSON(args []string) string {
	if len(args) == 0 {
		return "[]"
	}
	return fmt.Sprintf("['%v']", strings.Join(args, "', '"))
}

func (tm tableManager) withAPICall(device, op, jsonOp string, args []string, configured bool) string {
	if !tm.jsapi || !configured {
		return op
	}
	return fmt.Sprintf("<button id=\"%v.%v\" onclick=\"%s('%v', '%v', %s)\">%v</button>", device, op, jsonOp, device, op, argsAsJSON(args), op)
}

func (tm tableManager) withAPIConditionalCall(
	op, cond webapi.Action) string {
	if !tm.jsapi {
		return op.Op
	}
	opArgs := argsAsJSON(op.Args)
	condArgs := argsAsJSON(cond.Args)
	return fmt.Sprintf("<button id=\"%v.%v\" onclick=\"runConditionally('%v', '%v', %s, '%v', '%v', %s)\">%v</button>",
		op.Device, op.Op,
		op.Device, op.Op, opArgs,
		cond.Device, cond.Op, condArgs,
		op.Device)
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
	jsonOp := "runOperation"
	if condition {
		jsonOp = "runCondition"
	}
	for _, op := range ops {
		pars, configured := args[op]
		help := opsHelp[op]
		row := table.Row{
			tm.withDivID(device, condition),
			tm.withAPICall(device, op, jsonOp, pars, configured),
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

func (tm tableManager) ConditionalOperations(cops []conditionalOps) table.Writer {
	tw := table.NewWriter()
	tw.SetTitle("Conditional Operations")
	tw.AppendHeader(table.Row{"Device", "Op Conditional", "Op", "Args", "Device", "Condition", "Condition Args"})
	for _, cop := range cops {
		row := table.Row{
			cop.op.Device,
			tm.withAPIConditionalCall(cop.op, cop.cond),
			tm.withAPICall(cop.op.Device, cop.op.Op, "runOperation", cop.op.Args, true),
			strings.Join(cop.op.Args, ", "),
			cop.cond.Device,
			tm.withAPICall(cop.cond.Device, cop.cond.Op, "runCondition", cop.cond.Args, true),
			strings.Join(cop.cond.Args, ", "),
		}
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

func (tm tableManager) statusRecordRow(sr *logging.StatusRecord) table.Row {
	return table.Row{sr.Schedule, sr.Device, sr.Op, sr.Due, sr.Pending.Round(time.Second), sr.Completed.Round(time.Second), sr.PreConditionCall(), sr.Status(), sr.ErrorMessage()}
}

func (tm tableManager) statusRecordHeader() table.Row {
	return table.Row{"Schedule", "Device", "Operation", "Due", "Pending Since", "Completed", "Precondition", "Status", "Error"}
}

func (tm tableManager) CompletedAndPending(sr *logging.StatusRecorder, when datetime.CalendarDate) table.Writer {
	tw := table.NewWriter()
	tw.SetTitle(fmt.Sprintf("Completed and Pending: %v", when))
	n := 0
	tw.AppendHeader(tm.statusRecordHeader())
	for sr := range sr.Completed() {
		tw.AppendRow(tm.statusRecordRow(sr))
		n++
	}
	for sr := range sr.Pending() {
		tw.AppendRow(tm.statusRecordRow(sr))
		n++
	}
	if n == 0 {
		tw.SetTitle("No completed or pending operations")
	}
	return tw
}

func (tm tableManager) Completed(sr *logging.StatusRecorder, when datetime.CalendarDate) table.Writer {
	tw := table.NewWriter()
	tw.SetTitle(fmt.Sprintf("Completed: %v", when))
	tw.AppendHeader(tm.statusRecordHeader())
	for sr := range sr.Completed() {
		tm.statusRecordRow(sr)
	}
	return tw
}

func (tm tableManager) Pending(sr *logging.StatusRecorder, when datetime.CalendarDate) table.Writer {
	tw := table.NewWriter()
	tw.SetTitle(fmt.Sprintf("Pending: %v", when))
	tw.AppendHeader(tm.statusRecordHeader())
	for sr := range sr.Pending() {
		tm.statusRecordRow(sr)
	}
	return tw
}
