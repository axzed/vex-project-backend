package login_service_v1

import (
	"context"
	common "github.com/axzed/project-common"
	"github.com/axzed/project-common/encrypts"
	"github.com/axzed/project-common/errs"
	"github.com/axzed/project-common/jwts"
	"github.com/axzed/project-grpc/user/login"
	"github.com/axzed/project-user/config"
	"github.com/axzed/project-user/internal/dao"
	"github.com/axzed/project-user/internal/data"
	"github.com/axzed/project-user/internal/database/interface/conn"
	"github.com/axzed/project-user/internal/database/interface/transaction"
	"github.com/axzed/project-user/internal/repo"
	"github.com/axzed/project-user/pkg/model"
	"github.com/go-redis/redis/v8"
	"github.com/jinzhu/copier"
	"go.uber.org/zap"
	"log"
	"strconv"
	"time"
)

// LoginService 登录服务
type LoginService struct {
	login.UnimplementedLoginServiceServer
	cache            repo.Cache              // 缓存
	memberRepo       repo.MemberRepo         // 成员操作
	organizationRepo repo.OrganizationRepo   // 组织操作
	transaction      transaction.Transaction // 事务
}

// NewLoginService LoginService构造函数
func NewLoginService() *LoginService {
	return &LoginService{
		// 为定义的接口赋上实现类
		cache:            dao.Rc,
		memberRepo:       dao.NewMemberDao(),
		organizationRepo: dao.NewOrganizationDao(),
		transaction:      dao.NewTransactionImpl(),
	}
}

// GetCaptcha 获取验证码
func (ls *LoginService) GetCaptcha(ctx context.Context, msg *login.CaptchaMessage) (*login.CaptchaResponse, error) {
	// 1. 获取参数
	mobile := msg.Mobile
	// 2. 校验参数
	if !common.VerifyMobile(mobile) {
		return nil, errs.ConvertToGrpcError(model.ErrNotMobile)
	}
	// 3. 生成验证码 (随机4位1000-9999或者随机6位100000-999999)
	varifyCode := "123456"
	// 4. 调用短信平台 (三方 放入go协程中执行 不影响主流程 接口可以快速响应)
	go func() {
		time.Sleep(2 * time.Second)
		// test log
		zap.L().Info("短信平台调用成功，发送短信 INFO")
		// redis 假设后续缓存可能用MySQL, mongo, memcache当中的一种
		// 5. 将验证码存入redis (key:手机号 value:验证码 过期时间: 15分钟)log.Println("验证码发送成功: ")
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		err := ls.cache.Put(ctx, model.RegisterRedisKey+mobile, varifyCode, 15*time.Minute)
		if err != nil {
			log.Printf("验证码放入缓存失败, caused: %v\n", err)
		}
		log.Printf("将手机号和验证码存入redis成功: REGISTER_%s : %s\n", mobile, varifyCode)
	}()
	return &login.CaptchaResponse{Code: varifyCode}, nil
}

// Register 注册
func (ls *LoginService) Register(ctx context.Context, msg *login.RegisterMessage) (*login.RegisterResponse, error) {
	c := context.Background()
	// 1.可以再次进行参数校验
	// 2.校验验证码
	value, err := ls.cache.Get(c, model.RegisterRedisKey+msg.Mobile)
	// redis.Nil 代表key不存在
	// 草了，这个bug简直难受
	if err == redis.Nil {
		return nil, errs.ConvertToGrpcError(model.ErrCaptchNotFound)
	}
	if err != nil {
		zap.L().Error("Register redis get error", zap.Error(err))
		return nil, errs.ConvertToGrpcError(model.ErrRedisFail)
	}
	if value != msg.Captcha {
		return nil, errs.ConvertToGrpcError(model.ErrCaptcha)
	}
	// 3.校验业务逻辑(邮箱是否被注册 账号是否被注册 手机号是否被注册)
	exist, err := ls.memberRepo.GetMemberByEmail(c, msg.Email)
	if err != nil {
		zap.L().Error("Register db error", zap.Error(err))
		return nil, errs.ConvertToGrpcError(model.ErrDBFail)
	}
	if exist {
		return nil, errs.ConvertToGrpcError(model.ErrEmailExist)
	}

	exist, err = ls.memberRepo.GetMemberByAccount(c, msg.Name)
	if err != nil {
		zap.L().Error("Register db error", zap.Error(err))
		return nil, errs.ConvertToGrpcError(model.ErrDBFail)
	}
	if exist {
		return nil, errs.ConvertToGrpcError(model.ErrAccountExist)
	}

	exist, err = ls.memberRepo.GetMemberByMobile(c, msg.Mobile)
	if err != nil {
		zap.L().Error("Register db error", zap.Error(err))
		return nil, errs.ConvertToGrpcError(model.ErrDBFail)
	}
	if exist {
		return nil, errs.ConvertToGrpcError(model.ErrMobileExist)
	}
	// 4.执行业务逻辑 将数据存入member表 生成数据存入organization表
	// 4.1 将数据存入member表
	pwd := encrypts.Md5(msg.Password)
	mem := &data.Member{
		Account:       msg.Name,
		Password:      pwd,
		Name:          msg.Name,
		Mobile:        msg.Mobile,
		Email:         msg.Email,
		CreateTime:    time.Now().UnixMilli(),
		LastLoginTime: time.Now().UnixMilli(),
		Status:        model.Normal,
	}
	// 加入事务控制
	err = ls.transaction.Action(func(conn conn.DbConn) error {
		err = ls.memberRepo.SaveMember(conn, c, mem)
		if err != nil {
			zap.L().Error("Register db save error", zap.Error(err))
			return errs.ConvertToGrpcError(model.ErrDBFail)
		}
		// 4.2 生成数据存入organization表
		org := &data.Organization{
			Name:       mem.Name + "的个人项目",
			MemberId:   mem.Id,
			CreateTime: time.Now().UnixMilli(),
			Personal:   model.Personal,
			Avatar:     "https://gimg2.baidu.com/image_search/src=http%3A%2F%2Fc-ssl.dtstatic.com%2Fuploads%2Fblog%2F202103%2F31%2F20210331160001_9a852.thumb.1000_0.jpg&refer=http%3A%2F%2Fc-ssl.dtstatic.com&app=2002&size=f9999,10000&q=a80&n=0&g=0n&fmt=auto?sec=1673017724&t=ced22fc74624e6940fd6a89a21d30cc5",
		}
		err = ls.organizationRepo.SaveOrganization(conn, c, org)
		if err != nil {
			zap.L().Error("register SaveOrganization db err", zap.Error(err))
			return errs.ConvertToGrpcError(model.ErrDBFail)
		}
		return nil
	})

	// 5.返回结果
	return &login.RegisterResponse{}, nil
}

// Login 登录
func (ls *LoginService) Login(ctx context.Context, msg *login.LoginMessage) (*login.LoginResponse, error) {
	c := context.Background()
	// 1. 去数据库查询账号密码 记得密码要先加密
	pwd := encrypts.Md5(msg.Password)
	mem, err := ls.memberRepo.FindMember(c, msg.Account, pwd)
	if err != nil {
		zap.L().Error("Login db error", zap.Error(err))
		return nil, errs.ConvertToGrpcError(model.ErrDBFail)
	}
	if mem == nil {
		return nil, errs.ConvertToGrpcError(model.ErrAccountOrPwd)
	}
	memMessage := &login.MemberMessage{}
	err = copier.Copy(memMessage, mem)
	// 2. 根据用户ID去查询对应的组织
	orgs, err := ls.organizationRepo.FindOrganizationByMemberId(c, mem.Id)
	if err != nil {
		zap.L().Error("Login db error", zap.Error(err))
		return nil, errs.ConvertToGrpcError(model.ErrDBFail)
	}
	var orgsMessage []*login.OrganizationMessage
	err = copier.Copy(&orgsMessage, orgs)
	// 3. 用jwt生成token
	memIdStr := strconv.FormatInt(mem.Id, 10)
	exp := time.Duration(config.AppConf.JwtConfig.AccessExp*3600*24) * time.Second
	rExp := time.Duration(config.AppConf.JwtConfig.RefreshExp*3600*24) * time.Second
	token := jwts.CreateToken(memIdStr, exp, config.AppConf.JwtConfig.AccessSecret, rExp, config.AppConf.JwtConfig.RefreshSecret)
	tokenList := &login.TokenMessage{
		AccessToken:    token.AccessToken,
		RefreshToken:   token.RefreshToken,
		AccessTokenExp: token.AccessExp,
		TokenType:      "bearer",
	}
	// 4. 返回结果
	return &login.LoginResponse{
		Member:           memMessage,
		OrganizationList: orgsMessage,
		TokenList:        tokenList,
	}, nil
}
