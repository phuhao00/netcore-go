// Package rpc 编解码器实现
// Author: NetCore-Go Team
// Created: 2024

package rpc

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/vmihailenco/msgpack/v5"
	"google.golang.org/protobuf/proto"
)

// Codec 编解码器接口
type Codec interface {
	Encode(v interface{}) ([]byte, error)
	Decode(data interface{}, v interface{}) error
	Name() string
}

// JSONCodec JSON编解码器
type JSONCodec struct{}

// NewJSONCodec 创建JSON编解码器
func NewJSONCodec() *JSONCodec {
	return &JSONCodec{}
}

// Encode JSON编码
func (c *JSONCodec) Encode(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}

// Decode JSON解码
func (c *JSONCodec) Decode(data interface{}, v interface{}) error {
	// 如果data是[]byte，直接解码
	if bytes, ok := data.([]byte); ok {
		return json.Unmarshal(bytes, v)
	}
	
	// 如果data是其他类型，先编码再解码
	bytes, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal data: %v", err)
	}
	
	return json.Unmarshal(bytes, v)
}

// Name 获取编解码器名称
func (c *JSONCodec) Name() string {
	return "json"
}

// GobCodec Gob编解码器
type GobCodec struct{}

// NewGobCodec 创建Gob编解码器
func NewGobCodec() *GobCodec {
	return &GobCodec{}
}

// Encode Gob编码
func (c *GobCodec) Encode(v interface{}) ([]byte, error) {
	var buf bytes.Buffer
	encoder := gob.NewEncoder(&buf)
	if err := encoder.Encode(v); err != nil {
		return nil, fmt.Errorf("gob encode error: %v", err)
	}
	return buf.Bytes(), nil
}

// Decode Gob解码
func (c *GobCodec) Decode(data interface{}, v interface{}) error {
	var buf *bytes.Buffer
	
	// 如果data是[]byte，创建buffer
	if byteData, ok := data.([]byte); ok {
		buf = bytes.NewBuffer(byteData)
	} else {
		// 如果data是其他类型，先编码
		encoded, err := c.Encode(data)
		if err != nil {
			return fmt.Errorf("failed to encode data: %v", err)
		}
		buf = bytes.NewBuffer(encoded)
	}
	
	decoder := gob.NewDecoder(buf)
	if err := decoder.Decode(v); err != nil {
		return fmt.Errorf("gob decode error: %v", err)
	}
	
	return nil
}

// Name 获取编解码器名称
func (c *GobCodec) Name() string {
	return "gob"
}

// ProtobufCodec Protobuf编解码器
type ProtobufCodec struct{}

// NewProtobufCodec 创建Protobuf编解码器
func NewProtobufCodec() *ProtobufCodec {
	return &ProtobufCodec{}
}

// Encode Protobuf编码
func (c *ProtobufCodec) Encode(v interface{}) ([]byte, error) {
	msg, ok := v.(proto.Message)
	if !ok {
		return nil, fmt.Errorf("value must implement proto.Message")
	}
	
	return proto.Marshal(msg)
}

// Decode Protobuf解码
func (c *ProtobufCodec) Decode(data interface{}, v interface{}) error {
	msg, ok := v.(proto.Message)
	if !ok {
		return fmt.Errorf("value must implement proto.Message")
	}
	
	var bytes []byte
	if b, ok := data.([]byte); ok {
		bytes = b
	} else {
		// 如果data不是[]byte，尝试编码
		encoded, err := c.Encode(data)
		if err != nil {
			return fmt.Errorf("failed to encode data: %v", err)
		}
		bytes = encoded
	}
	
	return proto.Unmarshal(bytes, msg)
}

// Name 获取编解码器名称
func (c *ProtobufCodec) Name() string {
	return "protobuf"
}

// MsgPackCodec MessagePack编解码器（占位符）
type MsgPackCodec struct{}

// NewMsgPackCodec 创建MessagePack编解码器
func NewMsgPackCodec() *MsgPackCodec {
	return &MsgPackCodec{}
}

// Encode MessagePack编码
func (c *MsgPackCodec) Encode(v interface{}) ([]byte, error) {
	data, err := msgpack.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("msgpack encode error: %v", err)
	}
	return data, nil
}

// Decode MessagePack解码
func (c *MsgPackCodec) Decode(data interface{}, v interface{}) error {
	var bytes []byte
	
	// 如果data是[]byte，直接使用
	if byteData, ok := data.([]byte); ok {
		bytes = byteData
	} else {
		// 如果data是其他类型，先编码
		encoded, err := c.Encode(data)
		if err != nil {
			return fmt.Errorf("failed to encode data: %v", err)
		}
		bytes = encoded
	}
	
	if err := msgpack.Unmarshal(bytes, v); err != nil {
		return fmt.Errorf("msgpack decode error: %v", err)
	}
	
	return nil
}

// Name 获取编解码器名称
func (c *MsgPackCodec) Name() string {
	return "msgpack"
}

// CodecManager 编解码器管理器
type CodecManager struct {
	codecs map[string]Codec
}

// NewCodecManager 创建编解码器管理器
func NewCodecManager() *CodecManager {
	manager := &CodecManager{
		codecs: make(map[string]Codec),
	}
	
	// 注册默认编解码器
	manager.Register(NewJSONCodec())
	manager.Register(NewGobCodec())
	manager.Register(NewProtobufCodec())
	manager.Register(NewMsgPackCodec())
	
	return manager
}

// Register 注册编解码器
func (m *CodecManager) Register(codec Codec) {
	m.codecs[codec.Name()] = codec
}

// Get 获取编解码器
func (m *CodecManager) Get(name string) (Codec, error) {
	codec, exists := m.codecs[name]
	if !exists {
		return nil, fmt.Errorf("codec not found: %s", name)
	}
	return codec, nil
}

// List 列出所有编解码器
func (m *CodecManager) List() []string {
	names := make([]string, 0, len(m.codecs))
	for name := range m.codecs {
		names = append(names, name)
	}
	return names
}

// AutoDetectCodec 自动检测编解码器
func AutoDetectCodec(v interface{}) Codec {
	// 检查是否实现了proto.Message
	if _, ok := v.(proto.Message); ok {
		return NewProtobufCodec()
	}
	
	// 检查类型
	valueType := reflect.TypeOf(v)
	if valueType != nil {
		switch valueType.Kind() {
		case reflect.Struct, reflect.Map, reflect.Slice:
			// 复杂类型使用JSON
			return NewJSONCodec()
		default:
			// 简单类型使用Gob
			return NewGobCodec()
		}
	}
	
	// 默认使用JSON
	return NewJSONCodec()
}