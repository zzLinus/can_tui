package main

// A simple example that shows how to render an animated progress bar. In this
// example we bump the progress by 25% every two seconds, animating our
// progress bar to its new target state.
//
// It's also possible to render a progress bar in a more static fashion without
// transitions. For details on that approach see the progress-static example.

import (
	"container/ring"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	padding                = 2
	timerView sessionState = iota
	spinnerView
	maxWidth  = 80
	MAX_SPEED = 0x0aff
)

var (
	debug_s  string
	w, h     int
	motor_id [10]int
	can_pkg  [8]uint8
	can_r    *ring.Ring
	// Available spinners
	appStyle = lipgloss.NewStyle().Padding(1, 2)
	spinners = []spinner.Spinner{
		spinner.Line,
		spinner.Dot,
		spinner.MiniDot,
		spinner.Jump,
		spinner.Pulse,
		spinner.Points,
		spinner.Globe,
		spinner.Moon,
		spinner.Monkey,
	}

	titleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#fbf1c7")).
			Background(lipgloss.Color("#bd93f9")).
			Padding(0, 1)

	onlineStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#458588")).
			Background(lipgloss.Color("#50fa7b")).
			Padding(0, 1)

	offlineStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#458588")).
			Background(lipgloss.Color("#ff5555")).
			Padding(0, 1)

	modelStyle = lipgloss.NewStyle().
			Width(110).
			Height(50).
			Align(lipgloss.Center, lipgloss.Center).
			BorderStyle(lipgloss.HiddenBorder())
	focusedModelStyle = lipgloss.NewStyle().
				Width(110).
				Height(50).
				Align(lipgloss.Center, lipgloss.Center).
				BorderStyle(lipgloss.NormalBorder()).
				BorderForeground(lipgloss.Color("69"))

	spinnerStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("69"))
	helpStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("241")).PaddingLeft(3)
)

type editorFinishedMsg struct{ err error }

func main() {
	m := model{
		progress: progress.New(progress.WithDefaultGradient()),
		spinner:  spinner.New(),
	}

	m.spinner.Style = spinnerStyle
	m.spinner.Spinner = spinners[4]

	if _, err := tea.NewProgram(m).Run(); err != nil {
		fmt.Println("Oh no!", err)
		os.Exit(1)
	}
}

type tickMsg time.Time

type sessionState uint

type model struct {
	state    sessionState
	spinner  spinner.Model
	progress progress.Model
}

func (m model) Init() tea.Cmd {
	can_r = ring.New(5)
	return tea.Batch(tea.EnterAltScreen, tickCmd(), m.spinner.Tick)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.Type == tea.KeyPgUp {
			cmd := m.progress.IncrPercent(0.05)
			return m, tea.Batch(cmd)
		} else if msg.Type == tea.KeyPgDown {
			cmd := m.progress.DecrPercent(0.05)
			return m, tea.Batch(cmd)
		} else if msg.String() == "tab" {
			if m.state == timerView {
				m.state = spinnerView
			} else {
				m.state = timerView
			}

		} else if msg.String() == "q" {
			return m, tea.Quit
		} else {
			return m, nil
		}

	case tea.WindowSizeMsg:
		w = msg.Width
		h = msg.Height
		modelStyle = lipgloss.NewStyle().
			Width(w/2-2).
			Height(h-6).
			Align(lipgloss.Center, lipgloss.Center).
			BorderStyle(lipgloss.HiddenBorder())
		focusedModelStyle = lipgloss.NewStyle().
			Width(w/2-2).
			Height(h-6).
			Align(lipgloss.Center, lipgloss.Center).
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("69"))

		m.progress.Width = msg.Width - padding*2 - 4
		if m.progress.Width > maxWidth {
			m.progress.Width = maxWidth
		}
		return m, nil

	case spinner.TickMsg:
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case tickMsg:
		m.spinner, cmd = m.spinner.Update(msg)

		// Note that you can also use progress.Model.SetPercent to set the
		// percentage value explicitly, too.
		//cmd := m.progress.IncrPercent(0.25)
		return m, tea.Batch(tickCmd(), cansend(5, (int)(MAX_SPEED*m.progress.Percent())), cmd)

	// FrameMsg is sent when the progress bar wants to animate itself
	case progress.FrameMsg:
		progressModel, cmd := m.progress.Update(msg)
		m.progress = progressModel.(progress.Model)
		return m, cmd

	default:
		return m, nil
	}
	return m, nil
}

func (m model) currentFocusedModel() string {
	if m.state == timerView {
		return "timer"
	}
	return "spinner"
}

func (m model) View() string {
	var s, can_buf string
	motor_s := titleStyle.Render("Motor status") + "\n\n"
	pad := strings.Repeat(" ", padding)
	pad2 := strings.Repeat(" ", padding*34+1)
	var IDs []string
	for _, i := range motor_id {
		IDs = append(IDs, strconv.Itoa(i))
	}
	debug := fmt.Sprintf("%f",m.progress.Percent())+
		pad + debug_s + "min" + pad2 + "max\n\n"

	cmd := exec.Command("candump", "can0")

	out, err := cmd.StdoutPipe()
	if err != nil {
		panic(err)
	}
	if err := cmd.Start(); err != nil {
		panic(err)
	}
	buf := make([]byte, 42)
	n, err := out.Read(buf)
	if err != nil {
		panic(err)
	}

	can_r.Value = fmt.Sprintln(string(buf[0:n]))

	for i := 0; i < 4; i++ {
		if strings.Contains(fmt.Sprintln(string(buf[0:n])), fmt.Sprintf("20%d", i+1)) {
			motor_s += fmt.Sprintf("Motor %d : %s\n", i, onlineStyle.Render(" ONLINE👌"))
		} else {
			motor_s += fmt.Sprintf("Motor %d : %s\n", i, offlineStyle.Render("OFFLINE🚫"))
		}
	}

	can_r = can_r.Next()

	can_buf += "\n\n"
	can_r.Do(func(s any) {
		can_buf += fmt.Sprintln(s)
	})
	can_buf += "\n"

	if m.state == timerView {
		s += lipgloss.JoinHorizontal(lipgloss.Top, focusedModelStyle.Render(debug, m.progress.View()),
			modelStyle.Render(motor_s, can_buf, m.spinner.View(),
				m.spinner.View(), m.spinner.View(), m.spinner.View(), m.spinner.View(), m.spinner.View()))
	} else {
		s += lipgloss.JoinHorizontal(lipgloss.Top, modelStyle.Render(m.progress.View()),
			focusedModelStyle.Render(motor_s, can_buf, m.spinner.View(),
				m.spinner.View(), m.spinner.View(), m.spinner.View(), m.spinner.View(), m.spinner.View()))
	}
	s += helpStyle.Render("\ntab: focus next • q: exit\n")

	return s
}

func cansend(num int, speed int) tea.Cmd {
	debug_s = fmt.Sprintf("set speed : %d\n", speed)

	for i := 0; i < 4; i++ {
		if num&(1<<i) != 0 {
			can_pkg[2*i] = (uint8)(speed >> 8)
			can_pkg[2*i+1] = (uint8)(speed)
		}
	}

	debug_s += fmt.Sprintf("  can_pkg : %02x %02x %02x %02x %02x %02x %02x %02x\n\n",
		can_pkg[0], can_pkg[1], can_pkg[2], can_pkg[3],
		can_pkg[4], can_pkg[5], can_pkg[6], can_pkg[7])
	can_s := fmt.Sprintf("%02x%02x%02x%02x%02x%02x%02x%02x",
		can_pkg[0], can_pkg[1], can_pkg[2], can_pkg[3],
		can_pkg[4], can_pkg[5], can_pkg[6], can_pkg[7])

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
