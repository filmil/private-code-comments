package pkg

import (
	"database/sql"
	"fmt"
	"reflect"
	"sync"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

var (
	m sync.Mutex
	i uint
)

// DBName returns a unique database name each time it is called.
func DBName() string {
	m.Lock()
	defer m.Unlock()
	i++
	//db, err := sql.Open(SqliteDriver, "hux.sqlite?_pragma=foreign_keys(1)")
	return fmt.Sprintf("file:test_%d.db?cache=shared&mode=memory", i)
}

// NewDB creates a new test database, which is fully set up with the appropriate
// data schema for the test.
func NewDB() *sql.DB {
	n := DBName()
	db, err := sql.Open(SqliteDriver, n)
	if err != nil {
		panic(fmt.Sprintf("could not open database: %v", err))
	}
	if err := CreateSchema(db); err != nil {
		panic(fmt.Sprintf("could not create database schema: %v", err))
	}
	return db
}

// Must forces the arg call to pass.
func Must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Errorf("error: %v", err)
	}
}

func TestInsertRead(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name            string
		workspace, path string
		line            uint32
		content         string

		// What's expected on the output side.
		expected string
	}{
		{
			name:      "initial",
			workspace: "workspace",
			path:      "file_path",
			line:      42,
			content:   "hello",

			expected: "hello",
		},
	}

	for _, test := range tests {
		db := NewDB()
		test := test
		t.Run(test.name, func(t *testing.T) {
			if err := InsertAnn(db, test.workspace, test.path, test.line, test.content); err != nil {
				t.Fatalf("could not insert record:\n\t%v:\n\ttest=%+v", err, test)
			}

			actual, err := GetAnn(db, test.workspace, test.path, test.line)
			if err != nil {
				t.Fatalf("could not read record:\n\t%v:\n\ttest=%+v", err, test)
			}
			if actual != test.expected {
				t.Errorf("want: %+v\ngot:  %+v", test.expected, actual)
			}
		})
	}
}

func TestInsertDelete(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name            string
		workspace, path string
		line            uint32
		content         string

		// What's expected on the output side.
		expected string
	}{
		{
			name:      "initial",
			workspace: "workspace",
			path:      "file_path",
			line:      42,
			content:   "hello",
		},
	}

	for _, test := range tests {
		db := NewDB()

		test := test
		t.Run(test.name, func(t *testing.T) {
			if err := InsertAnn(db, test.workspace, test.path, test.line, test.content); err != nil {
				t.Fatalf("could not insert record:\n\t%v:\n\ttest=%+v", err, test)
			}

			err := DeleteAnn(db, test.workspace, test.path, test.line)
			if err != nil {
				t.Fatalf("could not delete record:\n\t%v:\n\ttest=%+v", err, test)
			}
		})
	}
}

func TestMove(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name            string
		workspace, path string
		line            uint32
		content         string

		newPath string
		newLine uint32

		// What's expected on the output side.
		expected string
	}{
		{
			name:      "initial",
			workspace: "workspace",
			path:      "file_path",
			line:      42,
			newPath:   "file_path_2",
			newLine:   142,
			content:   "hello",

			expected: "hello",
		},
	}

	for _, test := range tests {
		db := NewDB()

		test := test
		t.Run(test.name, func(t *testing.T) {
			if err := InsertAnn(db, test.workspace, test.path, test.line, test.content); err != nil {
				t.Fatalf("could not insert record:\n\t%v:\n\ttest=%+v", err, test)
			}

			err := MoveAnn(db, test.workspace, test.path, test.line, test.newPath, test.newLine)
			if err != nil {
				t.Fatalf("could not move record:\n\t%v:\n\ttest=%+v", err, test)
			}

			actual, err := GetAnn(db, test.workspace, test.newPath, test.newLine)
			if err != nil {
				t.Fatalf("could not GetAnn: %v: %v", err, test)
			}

			if actual != test.expected {
				t.Errorf("want: %+v\ngot:  %+v\n\t test: %+v", test.expected, actual, test)
			}
		})
	}
}

func TestBulkMove(t *testing.T) {
	db := NewDB()

	Must(t, InsertAnn(db, "ws", "path", 43, "one"))
	Must(t, InsertAnn(db, "ws", "path", 44, "two"))
	Must(t, InsertAnn(db, "ws", "path", 45, "three"))

	if err := BulkMoveAnn(db, "ws", "path", 44, 10); err != nil {
		t.Fatalf("error while move: %v", err)
	}

	actual, err := GetAnns(db, "ws", "path")
	if err != nil {
		t.Fatalf("could not GetAnns: %v", err)
	}

	want := []Ann{
		{43, "one"},
		{54, "two"},
		{55, "three"},
	}
	if reflect.DeepEqual(actual, want) == false {
		t.Errorf("want: %+v\n\tgot : %+v", want, actual)
	}
}

func TestInsertReadMulti(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		set       []Ann
		firstLine uint32
		delta     int32
		expected  []Ann
	}{
		{
			name: "first",
			set: []Ann{
				{Line: 1, Content: "one"},
				{Line: 10, Content: "ten"},
			},
			firstLine: 5,
			delta:     10,
			expected: []Ann{
				{Line: 1, Content: "one"},
				{Line: 20, Content: "ten"},
			},
		},
		{
			name: "first",
			set: []Ann{
				{Line: 1, Content: "one"},
				{Line: 10, Content: "ten"},
			},
			firstLine: 5,
			delta:     -5,
			expected: []Ann{
				{Line: 1, Content: "one"},
				{Line: 5, Content: "ten"},
			},
		},
		{
			name: "tricky",
			set: []Ann{
				{Line: 1, Content: "one"},
				{Line: 4, Content: "four"},
				{Line: 5, Content: "five"},
			},
			firstLine: 5,
			delta:     -5,
			expected: []Ann{
				// This is not strictly correct.
				{Line: 0, Content: "five"},
				{Line: 1, Content: "one"},
				{Line: 4, Content: "four"},
			},
		},
	}

	for _, test := range tests {
		db := NewDB()

		test := test
		t.Run(test.name, func(t *testing.T) {
			for _, i := range test.set {
				Must(t, InsertAnn(db, "ws", "path", i.Line, i.Content))
			}

			if err := BulkMoveAnn(db, "ws", "path", test.firstLine, test.delta); err != nil {
				t.Fatalf("could not bulk move: %v", err)
			}

			anns, err := GetAnns(db, "ws", "path")
			if err != nil {
				t.Fatalf("could not GetAnns: %v", err)
			}

			if reflect.DeepEqual(anns, test.expected) == false {
				t.Errorf("want: %+v\n\tgot  : %+v", test.expected, anns)
			}
		})
	}
}

func TestBulkRemove(t *testing.T) {
	tests := []struct {
		name      string
		set       []Ann
		firstLine uint32
		lastLine  uint32
		delta     int32
		expected  []Ann
	}{
		{
			name: "first",
			set: []Ann{
				{Line: 1, Content: "one"},
				{Line: 10, Content: "ten"},
			},
			firstLine: 5,
			lastLine:  6,
			delta:     10,
			expected: []Ann{
				{Line: 1, Content: "one"},
				{Line: 20, Content: "ten"},
			},
		},
		{
			name: "delete segment",
			set: []Ann{
				{Line: 1, Content: "one"},
				{Line: 10, Content: "ten"},
				{Line: 11, Content: "eleven"},
				{Line: 19, Content: "nineteen"},
				{Line: 20, Content: "twenty"},
			},
			firstLine: 11,
			lastLine:  19,
			delta:     -8,
			expected: []Ann{
				{Line: 1, Content: "one"},
				{Line: 10, Content: "ten"},
				{Line: 12, Content: "twenty"},
			},
		},
		{
			name: "replace segment",
			set: []Ann{
				{Line: 1, Content: "one"},
				{Line: 10, Content: "ten"},
				{Line: 11, Content: "eleven"},
				{Line: 19, Content: "nineteen"},
				{Line: 20, Content: "twenty"},
			},
			firstLine: 11,
			lastLine:  18,
			delta:     -5,
			expected: []Ann{
				{Line: 1, Content: "one"},
				{Line: 10, Content: "ten"},
				{Line: 14, Content: "nineteen"},
				{Line: 15, Content: "twenty"},
			},
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			db := NewDB()
			defer db.Close()
			for _, i := range test.set {
				Must(t, InsertAnn(db, "ws", "path", i.Line, i.Content))
			}

			if err := BulkDeleteAnn(db, "ws", "path", test.firstLine, test.lastLine, test.delta); err != nil {
				t.Fatalf("could not bulk move: %v", err)
			}

			anns, err := GetAnns(db, "ws", "path")
			if err != nil {
				t.Fatalf("could not GetAnns: %v", err)
			}

			if reflect.DeepEqual(anns, test.expected) == false {
				t.Errorf("want: %+v\n\tgot  : %+v", test.expected, anns)
			}
		})
	}
}
