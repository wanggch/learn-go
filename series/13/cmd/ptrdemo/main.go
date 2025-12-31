package main

import (
	"fmt"
	"math/rand"
)

type User struct {
	ID    int
	Name  string
	Score int
	Tags  []string
}

// valueProcessor returns a new slice, not mutating original.
func valueProcessor(users []User) []User {
	out := make([]User, len(users))
	copy(out, users)
	for i := range out {
		out[i].Score += 10
		out[i].Tags = append(out[i].Tags, "value-copy")
	}
	return out
}

// pointerProcessor mutates in place, saves allocations.
func pointerProcessor(users []*User) {
	for _, u := range users {
		if u == nil {
			continue
		}
		u.Score += 20
		u.Tags = append(u.Tags, "ptr-mutate")
	}
}

// comparePassing shows passing pointer vs value to a function.
func comparePassing(u User) {
	fmt.Printf("  [value] before: %+v\n", u)
	bumpValue(u)
	fmt.Printf("  [value] after bumpValue: %+v (unchanged)\n", u)

	fmt.Printf("  [pointer] before: %+v\n", u)
	bumpPointer(&u)
	fmt.Printf("  [pointer] after bumpPointer: %+v (mutated copy)\n", u)
}

func bumpValue(u User) {
	u.Score += 5
	u.Tags = append(u.Tags, "bumpValue")
}

func bumpPointer(u *User) {
	u.Score += 5
	u.Tags = append(u.Tags, "bumpPointer")
}

// largeStructDemo shows allocation difference.
type Payload struct {
	Data [1024]byte
}

func processValue(p Payload) {
	_ = p.Data[0]
}

func processPointer(p *Payload) {
	_ = p.Data[0]
}

func main() {
	fmt.Println("=== 指针 vs 值传递演示 ===")
	users := []User{
		{ID: 1, Name: "alice", Score: 90, Tags: []string{"original"}},
		{ID: 2, Name: "bob", Score: 85, Tags: []string{"original"}},
	}

	fmt.Println("\n1) 值拷贝 vs 指针原地修改")
	newUsers := valueProcessor(users)
	fmt.Printf("  原始 users[0]: %+v\n", users[0])
	fmt.Printf("  拷贝 newUsers[0]: %+v\n", newUsers[0])

	ptrs := []*User{&users[0], &users[1]}
	pointerProcessor(ptrs)
	fmt.Printf("  指针修改后 users[0]: %+v\n", users[0])

	fmt.Println("\n2) 函数参数：值 vs 指针")
	comparePassing(users[0])

	fmt.Println("\n3) 大对象传参：值会拷贝，指针避免额外复制")
	payload := Payload{}
	processValue(payload)
	processPointer(&payload)
	fmt.Println("  处理完成（观察 go build -gcflags=-m 可看到逃逸）")

	fmt.Println("\n4) 结构体切片重用：指针可避免重复查找")
	cache := map[int]*User{}
	getOrCreate := func(id int) *User {
		if u, ok := cache[id]; ok {
			return u
		}
		u := &User{ID: id, Name: fmt.Sprintf("user-%d", id)}
		cache[id] = u
		return u
	}
	for i := 0; i < 3; i++ {
		uid := 100 + rand.Intn(2)
		u := getOrCreate(uid)
		u.Score++
	}
	for id, u := range cache {
		fmt.Printf("  cache[%d]=%+v\n", id, u)
	}
}
