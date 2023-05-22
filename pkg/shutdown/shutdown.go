package shutdown

import (
	"os"
	"os/signal"
	"syscall"
)

// 这里为了确保hook实现了Hook接口，在编译时检查这种实现是否正确，提高代码的可维护性和健壮性。
var _ Hook = (*hook)(nil)

// Hook a graceful shutdown hook, default with signals of SIGINT and SIGTERM
type Hook interface {
	// WithSignals add more signals into hook
	WithSignals(signals ...syscall.Signal) Hook

	// Close register shutdown handles
	Close(funcs ...func())
}

type hook struct {
	ctx chan os.Signal
}

// NewHook create a Hook instance
func NewHook() Hook {
	hook := &hook{
		ctx: make(chan os.Signal, 1),
	}
	// 监听信号
	return hook.WithSignals(syscall.SIGINT, syscall.SIGTERM)
}

func (h *hook) WithSignals(signals ...syscall.Signal) Hook {
	// 监听信号，如果有信号传入，就会往ctx中写入数据
	for _, s := range signals {
		signal.Notify(h.ctx, s)
	}

	return h
}

func (h *hook) Close(funcs ...func()) {
	// 从ctx中读取数据，如果没有数据，就会阻塞，读取到数据则说明有信号传入，就会执行funcs中的函数
	select {
	case <-h.ctx:
	}
	// 关闭监听
	signal.Stop(h.ctx)

	for _, f := range funcs {
		f()
	}
}
