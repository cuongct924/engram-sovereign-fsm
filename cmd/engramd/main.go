package main

import (
	"crypto/sha256"
	"fmt"

	"github.com/celestiaorg/smt"
)

func main() {
	// 1. Khởi tạo Storage (In-memory cho prototype)
	nodes := smt.NewSimpleMap()
	values := smt.NewSimpleMap()

	// 2. Khởi tạo cây với hàm băm (Sử dụng SHA256 cho demo, hoặc Poseidon cho ZK)
	tree := smt.NewSparseMerkleTree(nodes, values, sha256.New())

	// 3. Cập nhật giá trị (Key-Value)
	key := []byte("user_1")
	val := []byte("state_anchored")
	_, err := tree.Update(key, val)
	if err != nil {
		panic(err)
	}

	// 4. Lấy Root Hash (Cái này sẽ được commit vào Blockchain State)
	root := tree.Root()
	fmt.Printf("Current Root: %x\n", root)

	// 5. Tạo bằng chứng (Proof) - Để xác thực cho các node khác
	proof, err := tree.Prove(key)
	if err != nil {
		panic(err)
	}

	// 6. Xác thực bằng chứng
	valid := smt.VerifyProof(proof, root, key, val, sha256.New())
	fmt.Printf("Proof valid: %v\n", valid)
}
