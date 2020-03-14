package GoMybatis

import (
	"database/sql"

	"github.com/zhuxiujia/GoMybatis/tx"
	"github.com/zhuxiujia/GoMybatis/utils"
)

type SessionFactorySession struct {
	Session Session
	Factory *SessionFactory
}

func (it *SessionFactorySession) Id() string {
	if it.Session == nil {
		return ""
	}
	return it.Session.Id()
}
func (it *SessionFactorySession) Query(sqlorArgs string) ([]map[string][]byte, error) {
	if it.Session == nil {
		return nil, utils.NewError("SessionFactorySession", " can not run Id(),it.Session == nil")
	}
	return it.Session.Query(sqlorArgs)
}
func (it *SessionFactorySession) QueryNew(sqlorArgs string) (*sql.Rows, error) {
	if it.Session == nil {
		return nil, utils.NewError("SessionFactorySession", " can not run Id(),it.Session == nil")
	}
	return it.Session.QueryNew(sqlorArgs)
}
func (it *SessionFactorySession) Exec(sqlorArgs string) (*Result, error) {
	if it.Session == nil {
		return nil, utils.NewError("SessionFactorySession", " can not run Exec(),it.Session == nil")
	}
	return it.Session.Exec(sqlorArgs)
}

func (it *SessionFactorySession) QueryPrepare(sqlorArgs string, args ...interface{}) ([]map[string][]byte, error) {
	if it.Session == nil {
		return nil, utils.NewError("SessionFactorySession", " can not run Id(),it.Session == nil")
	}
	return it.Session.QueryPrepare(sqlorArgs, args...)
}
func (it *SessionFactorySession) QueryPrepareNew(sqlorArgs string, args ...interface{}) (*sql.Rows, error) {
	if it.Session == nil {
		return nil, utils.NewError("SessionFactorySession", " can not run Id(),it.Session == nil")
	}
	return it.Session.QueryPrepareNew(sqlorArgs, args...)
}
func (it *SessionFactorySession) ExecPrepare(sqlorArgs string, args ...interface{}) (*Result, error) {
	if it.Session == nil {
		return nil, utils.NewError("SessionFactorySession", " can not run Exec(),it.Session == nil")
	}
	return it.Session.ExecPrepare(sqlorArgs, args...)
}

func (it *SessionFactorySession) Rollback() error {
	if it.Session == nil {
		return utils.NewError("SessionFactorySession", " can not run Rollback(),it.Session == nil")
	}
	return it.Session.Rollback()
}
func (it *SessionFactorySession) Commit() error {
	if it.Session == nil {
		return utils.NewError("SessionFactorySession", " can not run Commit(),it.Session == nil")
	}
	return it.Session.Commit()
}
func (it *SessionFactorySession) Begin(p *tx.Propagation) error {
	if it.Session == nil {
		return utils.NewError("SessionFactorySession", " can not run Begin(),it.Session == nil")
	}
	return it.Session.Begin(p)
}
func (it *SessionFactorySession) Close() {
	var id = it.Id()
	var s, _ = it.Factory.SessionMap.Load(id)
	if s != nil {
		if it.Session != nil {
			it.Session.Close()
		}
		it.Factory.SessionMap.Delete(id)
	}
}

func (it *SessionFactorySession) LastPROPAGATION() *tx.Propagation {
	return it.Session.LastPROPAGATION()
}
