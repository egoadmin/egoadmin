//go:build e2e

package main

type emptyResponse struct{}

type loginCryptoResponse struct {
	KeyID       string `json:"keyId"`
	PublicKey   string `json:"publicKey"`
	ChallengeID string `json:"challengeId"`
	Nonce       string `json:"nonce"`
	Algorithm   string `json:"algorithm"`
}

type loginResponse struct {
	UserTyp      int32  `json:"userTyp"`
	Menus        string `json:"menus"`
	Token        string `json:"token"`
	RefreshToken string `json:"refreshToken"`
}

type centerInfoResponse struct {
	User userInfo `json:"user"`
}

type userInfo struct {
	ID         string   `json:"id"`
	Username   string   `json:"username"`
	Password   string   `json:"password"`
	Name       string   `json:"name"`
	Phone      string   `json:"phone"`
	Gender     int32    `json:"gender"`
	UserStatus int32    `json:"userStatus"`
	UserOnline int32    `json:"userOnline"`
	DeptID     string   `json:"deptId"`
	RoleIDs    []string `json:"roleIds"`
	RoleNames  []string `json:"roleNames"`
	DeptName   string   `json:"deptName"`
}

type apisResponse struct {
	Apis []apiInfo `json:"apis"`
}

type apiInfo struct {
	ID       string `json:"id"`
	FullPath string `json:"fullPath"`
}

type idResponse struct {
	ID string `json:"id"`
}

type deptResponse struct {
	Dept deptInfo `json:"dept"`
}

type deptsResponse struct {
	Depts []deptInfo `json:"depts"`
}

type checkDeleteResponse struct {
	IsAllow bool   `json:"isAllow"`
	Msg     string `json:"msg"`
}

type deptInfo struct {
	ID       string     `json:"id"`
	ParentID string     `json:"parentId"`
	DeptName string     `json:"deptName"`
	Leader   string     `json:"leader"`
	Phone    string     `json:"phone"`
	Email    string     `json:"email"`
	Remark   string     `json:"remark"`
	Status   int32      `json:"status"`
	Level    int32      `json:"level"`
	HasChild bool       `json:"hasChild"`
	Childs   []deptInfo `json:"childs"`
}

type roleResponse struct {
	Role roleInfo `json:"role"`
}

type roleListResponse struct {
	Total int32      `json:"total"`
	Roles []roleInfo `json:"roles"`
}

type roleAllResponse struct {
	Roles []roleInfo `json:"roles"`
}

type roleInfo struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Typ       int32    `json:"typ"`
	DataPerm  int32    `json:"dataPerm"`
	Desc      string   `json:"desc"`
	ViewMenus string   `json:"viewMenus"`
	Uses      string   `json:"uses"`
	Apis      []string `json:"apis"`
}

type userResponse struct {
	User userInfo `json:"user"`
}

type userListResponse struct {
	Total int32      `json:"total"`
	Users []userInfo `json:"users"`
}

type logListResponse struct {
	Count int32     `json:"count"`
	Logs  []logInfo `json:"logs"`
}

type logInfo struct {
	ID         string `json:"id"`
	UserID     string `json:"userId"`
	Username   string `json:"username"`
	ModuleName string `json:"moduleName"`
	Typ        string `json:"typ"`
	Title      string `json:"title"`
	URL        string `json:"url"`
	Method     string `json:"method"`
	ClientIP   string `json:"clientIp"`
	Params     string `json:"params"`
	Remark     string `json:"remark"`
}
