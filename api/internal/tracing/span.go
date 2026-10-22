// Package tracingは、各層が標準trace.Tracerで開始・終了するSpanの共通規約を提供する。
// 業務上のerror分類は各層が行い、このpackageはUsecaseやGORMへ依存しない。
package tracing

import (
	"context"
	"errors"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// Layerは、threadly.layer属性へ記録する層の固定値。
type Layer string

const (
	LayerHTTP       Layer = "http"
	LayerHandler    Layer = "handler"
	LayerUsecase    Layer = "usecase"
	LayerRepository Layer = "repository"
)

// Resultは、threadly.result属性へ記録する処理結果の固定分類。
type Result string

const (
	ResultSuccess         Result = "success"
	ResultInvalidInput    Result = "invalid_input"
	ResultUnauthenticated Result = "unauthenticated"
	ResultForbidden       Result = "forbidden"
	ResultNotFound        Result = "not_found"
	ResultConflict        Result = "conflict"
	ResultRejected        Result = "rejected"
	ResultInternalError   Result = "internal_error"
	ResultPanic           Result = "panic"
)

const (
	layerKey     = attribute.Key("threadly.layer")
	resultKey    = attribute.Key("threadly.result")
	errorTypeKey = attribute.Key("error.type")
)

// Startは、HTTP以外の層の入口SpanをSpanKindInternalで開始する。
// 戻り値のContextを後続処理へ渡し、親子関係を保つ。
func Start(ctx context.Context, tracer trace.Tracer, layer Layer, name string) (context.Context, trace.Span) {
	return tracer.Start(
		ctx,
		name,
		trace.WithSpanKind(trace.SpanKindInternal),
		trace.WithAttributes(layerKey.String(string(layer))),
	)
}

// StartServerは、HTTP全体を表すSpanをSpanKindServerで開始する。
func StartServer(ctx context.Context, tracer trace.Tracer, name string) (context.Context, trace.Span) {
	return tracer.Start(
		ctx,
		name,
		trace.WithSpanKind(trace.SpanKindServer),
		trace.WithAttributes(layerKey.String(string(LayerHTTP))),
	)
}

// Endは、結果分類を記録してSpanを終了する。recoveredがnilでなければpanicとして記録し、
// 終了後に同じ値でpanicを再送出する。呼び出し側はdefer内でrecover()を直接渡す。
func End(span trace.Span, result Result, recovered any) {
	if recovered != nil {
		result = ResultPanic
	}
	span.SetAttributes(resultKey.String(string(result)))
	if result == ResultInternalError || result == ResultPanic {
		// 元のerrorやpanic値は秘密情報を含み得るため、固定分類から作ったerrorだけを記録する。
		span.SetAttributes(errorTypeKey.String(string(result)))
		span.RecordError(errors.New(string(result)))
		span.SetStatus(codes.Error, string(result))
	}
	span.End()
	if recovered != nil {
		panic(recovered)
	}
}
