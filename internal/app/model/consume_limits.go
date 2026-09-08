package model

import "time"

// ConsumeLimits описывает, сколько сервер ждёт результат обработки батча от consumer'а.
//
// Ожидание не может быть бесконечным: полумёртвый клиент (например, после сбоя реконнекта на его
// стороне) иначе навсегда остаётся CONNECTED, удерживает партиции kafka-группы и молча копит lag.
// При этом бюджет должен учитывать заявленные клиентом параметры, чтобы не рвать легитимную долгую
// обработку: он считается как PerMessage * размер батча + Slack и ограничивается сверху Max.
type ConsumeLimits struct {
	// PerMessage — бюджет на одно сообщение. Берётся из Connect.consumeTimeoutSec клиента,
	// если тот его объявил, иначе из конфигурации сервера.
	PerMessage time.Duration
	// Slack — запас поверх расчётного бюджета. Сервер должен быть терпеливее клиента, чтобы
	// клиент успел сам отреагировать на свой таймаут и переподключиться.
	Slack time.Duration
	// Max — верхняя граница итогового бюджета.
	Max time.Duration
}

// WithPerMessage возвращает копию с бюджетом на сообщение, заявленным клиентом.
// Нулевое или отрицательное значение означает "клиент не объявил", тогда остаётся серверное.
func (l ConsumeLimits) WithPerMessage(d time.Duration) ConsumeLimits {
	if d > 0 {
		l.PerMessage = d
	}
	return l
}

// BatchTimeout возвращает бюджет ожидания результата для батча из messageCount сообщений.
// Нулевой результат означает "лимит не настроен", ожидание в этом случае не ограничивается.
func (l ConsumeLimits) BatchTimeout(messageCount int) time.Duration {
	if l.PerMessage <= 0 {
		return 0
	}
	if messageCount < 1 {
		messageCount = 1
	}
	timeout := l.PerMessage*time.Duration(messageCount) + l.Slack
	if l.Max > 0 && timeout > l.Max {
		return l.Max
	}
	return timeout
}
