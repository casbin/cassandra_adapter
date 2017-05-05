package cassandra_adapter

import (
	"github.com/gocql/gocql"
	"github.com/hsluoyz/casbin/model"
	"strings"
	"strconv"
)

type Adapter struct {
	hosts []string
	session *gocql.Session
}

func NewAdapter(hosts ...string) *Adapter {
	a := Adapter{}
	a.hosts = hosts
	return &a
}

func (a *Adapter) open() {
	cluster := gocql.NewCluster(a.hosts...)

	session, err := cluster.CreateSession()

	err = session.Query("CREATE KEYSPACE IF NOT EXISTS casbin WITH REPLICATION = { 'class' : 'SimpleStrategy', 'replication_factor' : 1 } AND DURABLE_WRITES =  true ").Exec()
	if err != nil {
		panic(err)
	}

	//session.Close()
	//
	//cluster.Keyspace = "casbin"
	//session, err = cluster.CreateSession()
	//if err != nil {
	//	panic(err)
	//}

	a.session = session
}

func (a *Adapter) close() {
	a.session.Close()
}

func (a *Adapter) createTable() {
	err := a.session.Query("CREATE TABLE IF NOT EXISTS casbin.policy (no text PRIMARY KEY, ptype text, v1 text, v2 text, v3 text, v4 text)").Exec()
	if err != nil {
		panic(err)
	}
}

func (a *Adapter) dropTable() {
	err := a.session.Query("DROP TABLE IF EXISTS casbin.policy").Exec()
	if err != nil {
		panic(err)
	}
}

func loadPolicyLine(line string, model model.Model) {
	if line == "" {
		return
	}

	tokens := strings.Split(line, ", ")

	key := tokens[0]
	sec := key[:1]
	model[sec][key].Policy = append(model[sec][key].Policy, tokens[1:])
}

func (a *Adapter) LoadPolicy(model model.Model) {
	a.open()
	defer a.close()

	var (
		ptype string
		v1    string
		v2    string
		v3    string
		v4    string
	)

	iter := a.session.Query(`SELECT ptype, v1, v2, v3, v4 FROM casbin.policy`).Iter()
	for iter.Scan(&ptype, &v1, &v2, &v3, &v4) {
		line := ptype
		if v1 != "" {
			line += ", " + v1
		}
		if v2 != "" {
			line += ", " + v2
		}
		if v3 != "" {
			line += ", " + v3
		}
		if v4 != "" {
			line += ", " + v4
		}

		loadPolicyLine(line, model)
		// log.Println(ptype, v1, v2, v3, v4)
	}

	if err := iter.Close(); err != nil {
		panic(err)
	}
}

func (a *Adapter) writeTableLine(no int, ptype string, rule []string) {
	line := "'" + strconv.Itoa(no) + "','" + ptype + "'"
	for i := range rule {
		line += ",'" + rule[i] + "'"
	}
	for i := 0; i < 4-len(rule); i++ {
		line += ",''"
	}

	err := a.session.Query("INSERT INTO casbin.policy (no,ptype,v1,v2,v3,v4) values(" + line + ")").Exec()
	if err != nil {
		panic(err)
	}
}

// SavePolicy saves policy to database.
func (a *Adapter) SavePolicy(model model.Model) {
	a.open()
	defer a.close()

	a.dropTable()
	a.createTable()

	no := 0
	for ptype, ast := range model["p"] {
		for _, rule := range ast.Policy {
			a.writeTableLine(no, ptype, rule)
			no += 1
		}
	}

	for ptype, ast := range model["g"] {
		for _, rule := range ast.Policy {
			a.writeTableLine(no, ptype, rule)
			no += 1
		}
	}
}