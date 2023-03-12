package project

import (
	"context"
	"fmt"
	"github.com/axzed/project-api/api/rpc"
	"github.com/axzed/project-api/pkg/model"
	"github.com/axzed/project-api/pkg/model/param"
	common "github.com/axzed/project-common"
	"github.com/axzed/project-common/encrypts"
	"github.com/axzed/project-common/errs"
	"github.com/axzed/project-grpc/project"
	"github.com/gin-gonic/gin"
	"github.com/jinzhu/copier"
	"net/http"
	"strconv"
	"time"
)

// HandlerProject Handler接口实现类
type HandlerProject struct {
}

// NewHandlerProject NewHandlerUser
func NewHandlerProject() *HandlerProject {
	return &HandlerProject{}
}

// index 首页展示
func (p *HandlerProject) index(ctx *gin.Context) {
	result := &common.Result{}
	c, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	msg := &project.IndexMessage{}
	indexResponse, err := rpc.ProjectServiceClient.Index(c, msg)
	if err != nil {
		code, msg := errs.ParseGrpcError(err)
		ctx.JSON(http.StatusOK, result.Fail(code, msg))
	}
	menus := indexResponse.Menus
	var ms []*param.Menu
	copier.Copy(&ms, menus)
	ctx.JSON(http.StatusOK, result.Success(ms))
}

// myProjectList 我的项目列表
func (p *HandlerProject) myProjectList(ctx *gin.Context) {
	// 返回结果的结构体
	result := &common.Result{}
	// 1. 获取参数
	c, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	memberId, _ := ctx.Get("memberId")
	memberName := ctx.GetString("memberName")
	// 分页
	page := &model.Page{}
	page.Bind(ctx)
	selectBy := ctx.PostForm("selectBy")
	msg := &project.ProjectRpcMessage{
		MemberId:   memberId.(int64),
		MemberName: memberName,
		SelectBy:   selectBy,
		Page:       page.Page,
		PageSize:   page.PageSize,
	}
	myProjectResponse, err := rpc.ProjectServiceClient.FindProjectByMemId(c, msg)
	if err != nil {
		code, msg := errs.ParseGrpcError(err)
		ctx.JSON(http.StatusOK, result.Fail(code, msg))
	}

	var pms []*param.ProjectAndMember
	err = copier.Copy(&pms, myProjectResponse.Pm)
	// 若为空 设置默认值
	if pms == nil {
		pms = []*param.ProjectAndMember{}
	}
	if err != nil {
		ctx.JSON(http.StatusOK, result.Fail(http.StatusBadRequest, "copy参数失败"))
	}
	ctx.JSON(http.StatusOK, result.Success(gin.H{
		"list":  pms, // 不能返回null nil 前端无法解析 -> []
		"total": myProjectResponse.Total,
	}))
}

func (p *HandlerProject) projectTemplate(ctx *gin.Context) {
	// 返回结果的结构体
	result := &common.Result{}
	// 1. 获取参数
	c, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	memberId, _ := ctx.Get("memberId")
	memberName := ctx.GetString("memberName")
	// 分页
	page := &model.Page{}
	page.Bind(ctx)
	viewTypeStr := ctx.PostForm("viewType")
	viewType, _ := strconv.ParseInt(viewTypeStr, 10, 64)
	msg := &project.ProjectRpcMessage{
		MemberId:         memberId.(int64),
		MemberName:       memberName,
		ViewType:         int32(viewType),
		Page:             page.Page,
		PageSize:         page.PageSize,
		OrganizationCode: ctx.GetString("organizationCode"),
	}
	templateResponse, err := rpc.ProjectServiceClient.FindProjectTemplate(c, msg)
	if err != nil {
		code, msg := errs.ParseGrpcError(err)
		ctx.JSON(http.StatusOK, result.Fail(code, msg))
	}

	var pts []*param.ProjectTemplate
	err = copier.Copy(&pts, templateResponse.Ptm)
	// 若为空 设置默认值
	if pts == nil {
		pts = []*param.ProjectTemplate{}
	}
	if err != nil {
		ctx.JSON(http.StatusOK, result.Fail(http.StatusBadRequest, "copy参数失败"))
	}
	for _, v := range pts {
		// 若为空 设置默认值
		if v.TaskStages == nil {
			v.TaskStages = []*param.TaskStagesOnlyName{}
		}
	}
	ctx.JSON(http.StatusOK, result.Success(gin.H{
		"list":  pts, // 不能返回null nil 前端无法解析 -> []
		"total": templateResponse.Total,
	}))
}

// projectSave 项目保存(新建项目)
func (p *HandlerProject) projectSave(ctx *gin.Context) {
	// 返回结果的结构体
	result := &common.Result{}
	// 1. 获取参数
	c, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	memberId, _ := ctx.Get("memberId")
	organizationCodeStr := ctx.GetString("organizationCode")
	var req *param.SaveProjectRequest
	ctx.ShouldBind(&req)
	msg := &project.ProjectRpcMessage{
		MemberId:         memberId.(int64),
		OrganizationCode: organizationCodeStr,
		TemplateCode:     req.TemplateCode,
		Name:             req.Name,
		Id:               int64(req.Id),
		Description:      req.Description,
	}
	saveProject, err := rpc.ProjectServiceClient.SaveProject(c, msg)
	if err != nil {
		code, msg := errs.ParseGrpcError(err)
		ctx.JSON(http.StatusOK, result.Fail(code, msg))
	}
	var rsp *param.SaveProject
	copier.Copy(&rsp, saveProject)
	ctx.JSON(http.StatusOK, result.Success(rsp))
}

// readProject 项目详情
func (p *HandlerProject) readProject(ctx *gin.Context) {
	result := &common.Result{}
	projectCode := ctx.PostForm("projectCode")
	memberId := ctx.GetInt64("memberId")
	projectCodeStr, _ := encrypts.Decrypt(projectCode, "sdfgyrhgbxcdgryfhgywertd")
	fmt.Println(projectCodeStr)
	c, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	detail, err := rpc.ProjectServiceClient.FindProjectDetail(c, &project.ProjectRpcMessage{
		ProjectCode: projectCode,
		MemberId:    memberId,
	})
	if err != nil {
		code, msg := errs.ParseGrpcError(err)
		ctx.JSON(http.StatusOK, result.Fail(code, msg))
	}
	pd := &param.ProjectDetail{}
	copier.Copy(&pd, detail)
	ctx.JSON(http.StatusOK, result.Success(pd))
}
