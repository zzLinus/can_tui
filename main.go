package main

// A simple example that shows how to render an animated progress bar. In this
// example we bump the progress by 25% every two seconds, animating our
// progress bar to its new target state.
//
// It's also possible to render a progress bar in a more static fashion without
// transitions. For details on that approach see the progress-static example.

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	padding   = 2
	maxWidth  = 80
	MAX_SPEED = 0x08ff
)

var (
	debug_s  string
	motor_id [10]int
	can_pkg  [8]uint8
)

var helpStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#626262")).Render

type editorFinishedMsg struct{ err error }

func main() {
	m := model{
		progress: progress.New(progress.WithDefaultGradient()),
	}

	if _, err := tea.NewProgram(m).Run(); err != nil {
		fmt.Println("Oh no!", err)
		os.Exit(1)
	}
}

type tickMsg time.Time

type model struct {
	progress progress.Model
}

func (m model) Init() tea.Cmd {
	return tea.Batch(tea.EnterAltScreen, tickCmd())
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.Type == tea.KeyPgUp {
			cmd := m.progress.IncrPercent(0.05)
			return m, tea.Batch(cmd)
		} else if msg.Type == tea.KeyPgDown {
			cmd := m.progress.DecrPercent(0.05)
			return m, tea.Batch(cmd)

		} else {
			return m, tea.Quit
		}

	case tea.WindowSizeMsg:
		m.progress.Width = msg.Width - padding*2 - 4
		if m.progress.Width > maxWidth {
			m.progress.Width = maxWidth
		}
		return m, nil

	case tickMsg:
		if m.progress.Percent() == 1.0 {
			return m, nil
		}

		// Note that you can also use progress.Model.SetPercent to set the
		// percentage value explicitly, too.
		//cmd := m.progress.IncrPercent(0.25)
		return m, tea.Batch(tickCmd(), cansend(5, (int)(MAX_SPEED*m.progress.Percent())))

	// FrameMsg is sent when the progress bar wants to animate itself
	case progress.FrameMsg:
		progressModel, cmd := m.progress.Update(msg)
		m.progress = progressModel.(progress.Model)
		return m, cmd

	default:
		return m, nil
	}
}

func (m model) View() string {
	pad := strings.Repeat(" ", padding)
	pad2 := strings.Repeat(" ", padding*34+1)
	var IDs []string
	for _, i := range motor_id {
		IDs = append(IDs, strconv.Itoa(i))
	}
	return "\n" +
		pad + "min" + pad2 + "max\n\n" + pad + debug_s +
		pad + m.progress.View() + "\n\n" +
		pad + helpStyle("Press any key to quit")
}

func cansend(num int, speed int) tea.Cmd {
	debug_s = fmt.Sprintf("set speed : %d\n", speed)

	for i := 0; i < 4; i++ {
		if num&(1<<i) != 0 {
			can_pkg[2*i] = (uint8)(speed >> 8)
			can_pkg[2*i+1] = (uint8)(speed)
		}
	}

	debug_s += fmt.Sprintf("  can_pkg : %02x %02x %02x %02x %02x %02x %02x %02x\n", can_pkg[0], can_pkg[1], can_pkg[2], can_pkg[3], can_pkg[4], can_pkg[5], can_pkg[6], can_pkg[7])

	can_s := fmt.Sprintf("%02x%02x%02x%02x%02x%02x%02x%02x", can_pkg[0], can_pkg[1], can_pkg[2], can_pkg[3], can_pkg[4], can_pkg[5], can_pkg[6], can_pkg[7])

	editor := "cansend"
	// "01BB117001BBEE90"
	cmd := exec.Command(editor, "can0", "200#"+can_s)
	stdout, err := cmd.Output()
	if err != nil {
		fmt.Println(err.Error())
	}
	fmt.Print(string(stdout))

	return nil
}

func tickCmd() tea.Cmd {
	return tea.Tick(time.Microsecond*500, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}
