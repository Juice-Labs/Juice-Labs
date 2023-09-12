package logger

import (
	"fmt"
	"time"

	"go.uber.org/zap/buffer"
	"go.uber.org/zap/zapcore"

	pool "github.com/Juice-Labs/Juice-Labs/pkg/logger/pool"
)

// This is copy-pasta basically from zapcore.memory_encoder.go
type sliceArrayEncoder struct {
	elems []interface{}
}

func (s *sliceArrayEncoder) Clear() error {
	s.elems = s.elems[:0]
	return nil
}

func (s *sliceArrayEncoder) AppendArray(v zapcore.ArrayMarshaler) error {
	enc := &sliceArrayEncoder{}
	err := v.MarshalLogArray(enc)
	s.elems = append(s.elems, enc.elems)
	return err
}

func (s *sliceArrayEncoder) AppendObject(v zapcore.ObjectMarshaler) error {
	m := zapcore.NewMapObjectEncoder()
	err := v.MarshalLogObject(m)
	s.elems = append(s.elems, m.Fields)
	return err
}

func (s *sliceArrayEncoder) AppendReflected(v interface{}) error {
	s.elems = append(s.elems, v)
	return nil
}

func (s *sliceArrayEncoder) AppendBool(v bool)              { s.elems = append(s.elems, v) }
func (s *sliceArrayEncoder) AppendByteString(v []byte)      { s.elems = append(s.elems, string(v)) }
func (s *sliceArrayEncoder) AppendComplex128(v complex128)  { s.elems = append(s.elems, v) }
func (s *sliceArrayEncoder) AppendComplex64(v complex64)    { s.elems = append(s.elems, v) }
func (s *sliceArrayEncoder) AppendDuration(v time.Duration) { s.elems = append(s.elems, v) }
func (s *sliceArrayEncoder) AppendFloat64(v float64)        { s.elems = append(s.elems, v) }
func (s *sliceArrayEncoder) AppendFloat32(v float32)        { s.elems = append(s.elems, v) }
func (s *sliceArrayEncoder) AppendInt(v int)                { s.elems = append(s.elems, v) }
func (s *sliceArrayEncoder) AppendInt64(v int64)            { s.elems = append(s.elems, v) }
func (s *sliceArrayEncoder) AppendInt32(v int32)            { s.elems = append(s.elems, v) }
func (s *sliceArrayEncoder) AppendInt16(v int16)            { s.elems = append(s.elems, v) }
func (s *sliceArrayEncoder) AppendInt8(v int8)              { s.elems = append(s.elems, v) }
func (s *sliceArrayEncoder) AppendString(v string)          { s.elems = append(s.elems, v) }
func (s *sliceArrayEncoder) AppendTime(v time.Time)         { s.elems = append(s.elems, v) }
func (s *sliceArrayEncoder) AppendUint(v uint)              { s.elems = append(s.elems, v) }
func (s *sliceArrayEncoder) AppendUint64(v uint64)          { s.elems = append(s.elems, v) }
func (s *sliceArrayEncoder) AppendUint32(v uint32)          { s.elems = append(s.elems, v) }
func (s *sliceArrayEncoder) AppendUint16(v uint16)          { s.elems = append(s.elems, v) }
func (s *sliceArrayEncoder) AppendUint8(v uint8)            { s.elems = append(s.elems, v) }
func (s *sliceArrayEncoder) AppendUintptr(v uintptr)        { s.elems = append(s.elems, v) }

func singleLetterLevelEncoder(l zapcore.Level, enc zapcore.PrimitiveArrayEncoder) {

	single := "U"

	switch l {
	case zapcore.DebugLevel:
		single = "D"
	case zapcore.InfoLevel:
		single = "I"
	case zapcore.WarnLevel:
		single = "W"
	case zapcore.ErrorLevel:
		single = "E"
	case zapcore.DPanicLevel:
	case zapcore.PanicLevel:
		single = "P"
	case zapcore.FatalLevel:
		single = "F"
	}

	enc.AppendString(single)
}

// A zap encoder that follows the "juice" logging convention
// Writes out <data> <caller:line> <thread> <level>] <message>
type juiceEncoder struct {
	zapcore.Encoder
	*zapcore.EncoderConfig

	pool           buffer.Pool
	memoryEncoders *pool.Pool[*sliceArrayEncoder]
}

func NewJuiceEncoder(cfg zapcore.EncoderConfig) (zapcore.Encoder, error) {
	// We don't really allow customisation but we'll use some of the machinery
	cfg.ConsoleSeparator = " "
	cfg.EncodeLevel = singleLetterLevelEncoder

	encoder := juiceEncoder{
		Encoder:       zapcore.NewConsoleEncoder(cfg),
		EncoderConfig: &cfg,
		pool:          buffer.NewPool(),
		memoryEncoders: pool.New(func() *sliceArrayEncoder {
			return &sliceArrayEncoder{}
		}),
	}

	return encoder, nil
}

func (c juiceEncoder) Clone() zapcore.Encoder {
	return juiceEncoder{}
}

func (c juiceEncoder) EncodeEntry(ent zapcore.Entry, fields []zapcore.Field) (*buffer.Buffer, error) {
	line := c.pool.Get()

	arr := c.memoryEncoders.Get()

	c.EncodeTime(ent.Time, arr)

	if ent.Caller.Defined {
		c.EncodeCaller(ent.Caller, arr)
	}

	// TODO process:thread

	c.EncodeLevel(ent.Level, arr)

	for i := range arr.elems {
		if i > 0 {
			line.AppendString(c.ConsoleSeparator)
		}
		fmt.Fprint(line, arr.elems[i])
	}
	arr.Clear()

	line.AppendByte(']')
	line.AppendByte(' ')
	line.AppendString(ent.Message)

	line.AppendString(c.LineEnding)
	return line, nil
}
