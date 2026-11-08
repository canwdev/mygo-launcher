package main

import (
	"os"
	"path/filepath"
	"testing"
)

// TestTimerOffDoesNotTrack 验证计时器关闭后不再跟踪程序：同一程序可以再次
// 启动，计划里也不带跟踪。
func TestTimerOffDoesNotTrack(t *testing.T) {
	app := newTestApp(t)
	if err := os.WriteFile(filepath.Join(absBase, "tool.exe"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	app.store.Apps["x"] = &AppItem{GUID: "x", Name: "Tool", Path: "tool.exe"}
	app.runtimeStats["x"] = 0
	app.store.Settings.AutoTime = true

	plan, err := app.planLaunch("x")
	if err != nil || !plan.track {
		t.Fatalf("with the timer on the program is tracked: plan=%+v err=%v", plan, err)
	}
	app.running["x"] = &runningProc{timed: true}
	if _, err := app.planLaunch("x"); err != errAlreadyRunning {
		t.Fatalf("a running program is refused while timing, err=%v", err)
	}

	app.store.Settings.AutoTime = false
	plan, err = app.planLaunch("x")
	if err != nil {
		t.Fatalf("the same program starts again with the timer off: %v", err)
	}
	if plan.track {
		t.Fatalf("nothing is tracked with the timer off: %+v", plan)
	}
}

// TestStopTimingForgetsProcesses 验证关闭计时器会立即清空运行跟踪、停止计时
// 刷新，并让旧进程退出后不再补计时。
func TestStopTimingForgetsProcesses(t *testing.T) {
	app := newTestApp(t)
	app.store.Settings.AutoTime = true
	app.store.Apps["x"] = &AppItem{GUID: "x", Name: "Tool", Path: "tool.exe"}
	app.runtimeStats["x"] = 1000
	p := &runningProc{timed: true}
	app.running["x"] = p
	app.tick.Store(true)

	app.stopTiming()
	if len(app.running) != 0 {
		t.Fatalf("no process may run for the window, running=%v", app.running)
	}
	if p.timed {
		t.Fatal("a process running when the timer stops must not be timed")
	}
	if app.tick.Load() {
		t.Fatal("the clock must stop redrawing runtimes")
	}

	app.store.Settings.AutoTime = true
	app.finishProcess("x", p, 5000)
	if got := app.runtimeStats["x"]; got != 1000 {
		t.Fatalf("a process started before the timer stopped adds nothing, runtime=%d", got)
	}

	// The same item started again while the old process still runs: the old
	// exit must not drop the new process.
	again := &runningProc{timed: true}
	app.running["x"] = again
	app.finishProcess("x", p, 5000)
	if app.running["x"] != again {
		t.Fatal("the exit of an untracked process must not forget a newer one")
	}
}
