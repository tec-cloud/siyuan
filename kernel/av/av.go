// SiYuan - Refactor your thinking
// Copyright (c) 2020-present, b3log.org
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

// Package av 包含了属性视图（Attribute View）相关的实现。
package av

import (
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/88250/gulu"
	"github.com/88250/lute/ast"
	"github.com/siyuan-note/filelock"
	"github.com/siyuan-note/logging"
	"github.com/siyuan-note/siyuan/kernel/util"
)

// AttributeView 描述了属性视图的结构。
type AttributeView struct {
	Spec      int          `json:"spec"`      // 格式版本
	ID        string       `json:"id"`        // 属性视图 ID
	Name      string       `json:"name"`      // 属性视图名称
	KeyValues []*KeyValues `json:"keyValues"` // 属性视图属性列值
	ViewID    string       `json:"viewID"`    // 当前视图 ID
	Views     []*View      `json:"views"`     // 视图
}

// KeyValues 描述了属性视图属性列值的结构。
type KeyValues struct {
	Key    *Key     `json:"key"`              // 属性视图属性列
	Values []*Value `json:"values,omitempty"` // 属性视图属性列值
}

type KeyType string

const (
	KeyTypeBlock   KeyType = "block"
	KeyTypeText    KeyType = "text"
	KeyTypeNumber  KeyType = "number"
	KeyTypeDate    KeyType = "date"
	KeyTypeSelect  KeyType = "select"
	KeyTypeMSelect KeyType = "mSelect"
)

// Key 描述了属性视图属性列的基础结构。
type Key struct {
	ID   string  `json:"id"`   // 列 ID
	Name string  `json:"name"` // 列名
	Type KeyType `json:"type"` // 列类型
	Icon string  `json:"icon"` // 列图标

	// 以下是某些列类型的特有属性

	Options []*KeySelectOption `json:"options,omitempty"` // 选项列表
}

func NewKey(id, name string, keyType KeyType) *Key {
	return &Key{
		ID:   id,
		Name: name,
		Type: keyType,
	}
}

type KeySelectOption struct {
	Name  string `json:"name"`
	Color string `json:"color"`
}

type Value struct {
	ID      string  `json:"id,omitempty"`
	KeyID   string  `json:"keyID,omitempty"`
	BlockID string  `json:"blockID,omitempty"`
	Type    KeyType `json:"type,omitempty"`

	Block   *ValueBlock    `json:"block,omitempty"`
	Text    *ValueText     `json:"text,omitempty"`
	Number  *ValueNumber   `json:"number,omitempty"`
	Date    *ValueDate     `json:"date,omitempty"`
	MSelect []*ValueSelect `json:"mSelect,omitempty"`
}

func (value *Value) ToJSONString() string {
	data, err := gulu.JSON.MarshalJSON(value)
	if nil != err {
		return ""
	}
	return string(data)
}

type ValueBlock struct {
	ID      string `json:"id"`
	Content string `json:"content"`
}

type ValueText struct {
	Content string `json:"content"`
}

type ValueNumber struct {
	Content          float64      `json:"content"`
	IsNotEmpty       bool         `json:"isNotEmpty"`
	Format           NumberFormat `json:"format"`
	FormattedContent string       `json:"formattedContent"`
}

type NumberFormat string

const (
	NumberFormatNone    NumberFormat = ""
	NumberFormatPercent NumberFormat = "percent"
)

func NewValueNumber(content float64) *ValueNumber {
	return &ValueNumber{
		Content:          content,
		IsNotEmpty:       true,
		Format:           NumberFormatNone,
		FormattedContent: fmt.Sprintf("%f", content),
	}
}

func NewFormattedValueNumber(content float64, format NumberFormat) (ret *ValueNumber) {
	ret = &ValueNumber{
		Content:          content,
		IsNotEmpty:       true,
		Format:           format,
		FormattedContent: fmt.Sprintf("%f", content),
	}
	switch format {
	case NumberFormatNone:
		s := fmt.Sprintf("%.5f", content)
		ret.FormattedContent = strings.TrimRight(strings.TrimRight(s, "0"), ".")
	case NumberFormatPercent:
		s := fmt.Sprintf("%.2f", content*100)
		ret.FormattedContent = strings.TrimRight(strings.TrimRight(s, "0"), ".") + "%"
	}
	return
}

func (number *ValueNumber) FormatNumber() {
	switch number.Format {
	case NumberFormatNone:
		number.FormattedContent = strconv.FormatFloat(number.Content, 'f', -1, 64)
	case NumberFormatPercent:
		s := fmt.Sprintf("%.2f", number.Content*100)
		number.FormattedContent = strings.TrimRight(strings.TrimRight(s, "0"), ".") + "%"
	}
}

type ValueDate struct {
	Content          int64  `json:"content"`
	Content2         int64  `json:"content2"`
	HasEndDate       bool   `json:"hasEndDate"`
	FormattedContent string `json:"formattedContent"`
}

type DateFormat string

const (
	DateFormatNone     DateFormat = ""
	DateFormatDuration DateFormat = "duration"
)

func NewFormattedValueDate(content, content2 int64, format DateFormat) (ret *ValueDate) {
	formatted := time.UnixMilli(content).Format("2006-01-02 15:04")
	if 0 < content2 {
		formatted += " → " + time.UnixMilli(content2).Format("2006-01-02 15:04")
	}
	switch format {
	case DateFormatNone:
	case DateFormatDuration:
		t1 := time.UnixMilli(content)
		t2 := time.UnixMilli(content2)
		formatted = util.HumanizeRelTime(t1, t2, util.Lang)
	}
	ret = &ValueDate{
		Content:          content,
		Content2:         content2,
		HasEndDate:       false,
		FormattedContent: formatted,
	}
	return
}

// RoundUp rounds like 12.3416 -> 12.35
func RoundUp(val float64, precision int) float64 {
	return math.Ceil(val*(math.Pow10(precision))) / math.Pow10(precision)
}

// RoundDown rounds like 12.3496 -> 12.34
func RoundDown(val float64, precision int) float64 {
	return math.Floor(val*(math.Pow10(precision))) / math.Pow10(precision)
}

// Round rounds to nearest like 12.3456 -> 12.35
func Round(val float64, precision int) float64 {
	return math.Round(val*(math.Pow10(precision))) / math.Pow10(precision)
}

type ValueSelect struct {
	Content string `json:"content"`
	Color   string `json:"color"`
}

// View 描述了视图的结构。
type View struct {
	ID   string `json:"id"`   // 视图 ID
	Name string `json:"name"` // 视图名称

	LayoutType LayoutType   `json:"type"`            // 当前布局类型
	Table      *LayoutTable `json:"table,omitempty"` // 表格布局
}

// LayoutType 描述了视图布局的类型。
type LayoutType string

const (
	LayoutTypeTable LayoutType = "table" // 属性视图类型 - 表格
)

func NewView() *View {
	name := "Table"
	return &View{
		ID:         ast.NewNodeID(),
		Name:       name,
		LayoutType: LayoutTypeTable,
		Table: &LayoutTable{
			Spec:    0,
			ID:      ast.NewNodeID(),
			Filters: []*ViewFilter{},
			Sorts:   []*ViewSort{},
		},
	}
}

// Viewable 描述了视图的接口。
type Viewable interface {
	Filterable
	Sortable
	Calculable

	GetType() LayoutType
	GetID() string
}

func NewAttributeView(id string) (ret *AttributeView) {
	view := NewView()
	key := NewKey(ast.NewNodeID(), "Block", KeyTypeBlock)
	ret = &AttributeView{
		Spec:      0,
		ID:        id,
		KeyValues: []*KeyValues{{Key: key}},
		ViewID:    view.ID,
		Views:     []*View{view},
	}
	view.Table.Columns = []*ViewTableColumn{{ID: key.ID}}
	return
}

func ParseAttributeView(avID string) (ret *AttributeView, err error) {
	avJSONPath := getAttributeViewDataPath(avID)
	toCreate := false
	if !gulu.File.IsExist(avJSONPath) {
		ret = NewAttributeView(avID)
		toCreate = true
	}

	if !toCreate {
		data, readErr := filelock.ReadFile(avJSONPath)
		if nil != readErr {
			logging.LogErrorf("read attribute view [%s] failed: %s", avID, readErr)
			return
		}

		ret = &AttributeView{}
		if err = gulu.JSON.UnmarshalJSON(data, ret); nil != err {
			logging.LogErrorf("unmarshal attribute view [%s] failed: %s", avID, err)
			return
		}
	} else {
		if err = SaveAttributeView(ret); nil != err {
			logging.LogErrorf("save attribute view [%s] failed: %s", avID, err)
			return
		}
	}
	return
}

func SaveAttributeView(av *AttributeView) (err error) {
	data, err := gulu.JSON.MarshalIndentJSON(av, "", "\t") // TODO: single-line for production
	if nil != err {
		logging.LogErrorf("marshal attribute view [%s] failed: %s", av.ID, err)
		return
	}

	avJSONPath := getAttributeViewDataPath(av.ID)
	if err = filelock.WriteFile(avJSONPath, data); nil != err {
		logging.LogErrorf("save attribute view [%s] failed: %s", av.ID, err)
		return
	}
	return
}

func (av *AttributeView) GetView() (ret *View, err error) {
	for _, v := range av.Views {
		if v.ID == av.ViewID {
			ret = v
			return
		}
	}
	err = ErrViewNotFound
	return
}

func (av *AttributeView) GetKey(keyID string) (ret *Key, err error) {
	for _, kv := range av.KeyValues {
		if kv.Key.ID == keyID {
			ret = kv.Key
			return
		}
	}
	err = ErrKeyNotFound
	return
}

func (av *AttributeView) GetBlockKeyValues() (ret *KeyValues) {
	for _, kv := range av.KeyValues {
		if KeyTypeBlock == kv.Key.Type {
			ret = kv
			return
		}
	}
	return
}

func getAttributeViewDataPath(avID string) (ret string) {
	av := filepath.Join(util.DataDir, "storage", "av")
	ret = filepath.Join(av, avID+".json")
	if !gulu.File.IsDir(av) {
		if err := os.MkdirAll(av, 0755); nil != err {
			logging.LogErrorf("create attribute view dir failed: %s", err)
			return
		}
	}
	return
}

var (
	ErrViewNotFound = errors.New("view not found")
	ErrKeyNotFound  = errors.New("key not found")
)
