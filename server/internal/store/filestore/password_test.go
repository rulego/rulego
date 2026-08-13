package filestore

import "testing"

func TestHashPassword_RoundTrip(t *testing.T) {
	hashed, err := HashPassword("s3cret-pw")
	if err != nil {
		t.Fatalf("HashPassword error: %v", err)
	}
	if hashed == "s3cret-pw" {
		t.Fatal("密码未散列，仍是明文")
	}
	if !IsHashedPassword(hashed) {
		t.Fatalf("IsHashedPassword(%q) = false, want true", hashed)
	}
	if !VerifyPassword(hashed, "s3cret-pw") {
		t.Error("正确密码校验失败")
	}
	if VerifyPassword(hashed, "wrong-pw") {
		t.Error("错误密码被接受")
	}
}

func TestHashPassword_SaltIsRandom(t *testing.T) {
	a, err := HashPassword("same")
	if err != nil {
		t.Fatalf("HashPassword error: %v", err)
	}
	b, err := HashPassword("same")
	if err != nil {
		t.Fatalf("HashPassword error: %v", err)
	}
	if a == b {
		t.Error("相同密码两次散列结果相同，盐未随机")
	}
	if !VerifyPassword(a, "same") || !VerifyPassword(b, "same") {
		t.Error("两次散列都应能校验通过")
	}
}

func TestHashPassword_Empty(t *testing.T) {
	hashed, err := HashPassword("")
	if err != nil {
		t.Fatalf("HashPassword error: %v", err)
	}
	if hashed != "" {
		t.Errorf("空密码应原样返回，得到 %q", hashed)
	}
}

// 历史 users.ini 与 config.conf 内置账号是明文，必须继续可用
func TestVerifyPassword_LegacyPlaintext(t *testing.T) {
	if !VerifyPassword("plain123", "plain123") {
		t.Error("明文存储的正确密码应校验通过")
	}
	if VerifyPassword("plain123", "nope") {
		t.Error("明文存储的错误密码应拒绝")
	}
}

func TestVerifyPassword_Malformed(t *testing.T) {
	cases := []struct {
		name   string
		stored string
	}{
		{"空值", ""},
		{"只有前缀", pwHashPrefix},
		{"段数不足", pwHashPrefix + "210000$abcd"},
		{"迭代次数非法", pwHashPrefix + "abc$61626364$61626364"},
		{"迭代次数为零", pwHashPrefix + "0$61626364$61626364"},
		{"盐非十六进制", pwHashPrefix + "210000$zz$61626364"},
		{"散列非十六进制", pwHashPrefix + "210000$61626364$zz"},
		{"散列为空", pwHashPrefix + "210000$61626364$"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if VerifyPassword(c.stored, "anything") {
				t.Errorf("畸形散列 %q 不应校验通过", c.stored)
			}
		})
	}
}
