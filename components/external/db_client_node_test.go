/*
 * Copyright 2023 The RuleGo Authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package external

import (
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/test"
	"github.com/rulego/rulego/test/assert"
	"github.com/rulego/rulego/utils/json"
	"net"
	"testing"
	"time"
)

func TestDbClientNode(t *testing.T) {
	var targetNodeType = "dbClient"

	t.Run("NewNode", func(t *testing.T) {
		test.NodeNew(t, targetNodeType, &DbClientNode{}, types.Configuration{
			"sql":        "select * from test",
			"driverName": "mysql",
			"dsn":        "root:root@tcp(127.0.0.1:3306)/test",
		}, Registry)
	})
	t.Run("defaultConfig", func(t *testing.T) {
		node := &DbClientNode{}
		err := node.Init(types.NewConfig(), types.Configuration{})
		assert.NotNil(t, err)
		assert.Equal(t, "mysql", node.Config.DriverName)

		node2 := &DbClientNode{}
		err = node2.Init(types.NewConfig(), types.Configuration{
			"sql":        "xx",
			"driverName": "mysql",
			"dsn":        "root:root@tcp(127.0.0.1:3306)/test",
		})
		assert.Equal(t, "unsupported sql statement: xx", err.Error())
	})

	t.Run("OnMsgMysql", func(t *testing.T) {
		testDbClientNodeOnMsg(t, targetNodeType, "mysql", "root:root@tcp(127.0.1.1:3306)/test")
	})
	t.Run("OnMsgPostgres", func(t *testing.T) {
		testDbClientNodeOnMsg(t, targetNodeType, "postgres", "postgres://postgres:postgres@127.0.1.1:5432/test?sslmode=disable")
	})
	t.Run("TestConcurrency", func(t *testing.T) {
		testConcurrency(t, targetNodeType, "mysql", "root:root@tcp(127.0.1.1:3306)/test")
	})
}

func testDbClientNodeOnMsg(t *testing.T, targetNodeType, driverName, dsn string) {

	node1 := &DbClientNode{}
	err := node1.Init(types.NewConfig(), types.Configuration{})
	assert.NotNil(t, err)
	assert.Equal(t, "mysql", node1.Config.DriverName)

	node2, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
		"sql":        "insert into users (id,name, age) values (?,?,?)",
		"params":     []interface{}{"${metadata.id}", "${metadata.name}", 18},
		"driverName": driverName,
		"dsn":        dsn,
	}, Registry)

	node3, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
		"sql":        "update users set age = ? where id = ?",
		"params":     []interface{}{"${metadata.age}", "${metadata.id}"},
		"driverName": driverName,
		"dsn":        dsn,
	}, Registry)

	node4, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
		"sql":        "select id,name,age from users",
		"driverName": driverName,
		"dsn":        dsn,
	}, Registry)

	node5, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
		"sql":        "select * from users where id = ?",
		"params":     []interface{}{"${metadata.id}"},
		"getOne":     true,
		"driverName": driverName,
		"dsn":        dsn,
	}, Registry)
	node5_2, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
		"sql":        "${metadata.sql}",
		"params":     []interface{}{"${metadata.id}"},
		"getOne":     true,
		"driverName": driverName,
		"dsn":        dsn,
	}, Registry)
	node6, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
		"sql":        "delete from users",
		"params":     nil,
		"poolSize":   10,
		"driverName": driverName,
		"dsn":        dsn,
	}, Registry)

	metaData := types.BuildMetadata(make(map[string]string))
	metaData.PutValue("id", "1")
	metaData.PutValue("name", "test01")
	metaData.PutValue("age", "18")
	metaData.PutValue("sql", "select * from users where id = ?")

	updateMetaData := types.BuildMetadata(make(map[string]string))
	updateMetaData.PutValue("id", "1")
	updateMetaData.PutValue("name", "test01")
	updateMetaData.PutValue("age", "21")

	msgList := []test.Msg{
		{
			MetaData:   metaData,
			MsgType:    "ACTIVITY_EVENT2",
			Data:       "{\"temperature\":60}",
			AfterSleep: time.Millisecond * 200,
		},
	}

	var nodeList = []test.NodeAndCallback{
		{
			Node:    node1,
			MsgList: msgList,
			Callback: func(msg types.RuleMsg, relationType string, err error) {
				if err != nil {
					if _, ok := err.(*net.OpError); ok {
						// skip test
					}
				} else {
					assert.Equal(t, types.Failure, relationType)
					assert.Equal(t, "unsupported sql statement: ", err.Error())
				}

			},
		},
		{
			Node:    node2,
			MsgList: msgList,
			Callback: func(msg types.RuleMsg, relationType string, err error) {
				if err != nil {
					if _, ok := err.(*net.OpError); ok {
						// skip test
					} else {
						t.Fatal("bad", err.Error())
					}
				} else {
					assert.Equal(t, "1", msg.Metadata.GetValue(rowsAffectedKey))
				}
			},
		},
		{
			Node: node3,
			MsgList: []test.Msg{
				{
					MetaData:   updateMetaData,
					MsgType:    "ACTIVITY_EVENT2",
					Data:       "{\"temperature\":60}",
					AfterSleep: time.Millisecond * 200,
				},
			},
			Callback: func(msg types.RuleMsg, relationType string, err error) {
				if err != nil {
					if _, ok := err.(*net.OpError); ok {
						// skip test
					} else {
						t.Fatal("bad", err.Error())
					}
				} else {
					assert.Equal(t, "1", msg.Metadata.GetValue(rowsAffectedKey))
				}
			},
		},
		{
			Node:    node4,
			MsgList: msgList,
			Callback: func(msg types.RuleMsg, relationType string, err error) {
				if err != nil {
					if _, ok := err.(*net.OpError); ok {
						// skip test
					} else {
						t.Fatal("bad", err.Error())
					}
				} else {
					var list []testUser
					_ = json.Unmarshal([]byte(msg.GetData()), &list)
					assert.True(t, len(list) > 0)
					//assert.Equal(t, int64(1), list[0].Id)
					//assert.Equal(t, 21, list[0].Age)
					assert.Equal(t, "test01", list[0].Name)
				}
			},
		},
		{
			Node:    node5,
			MsgList: msgList,
			Callback: func(msg types.RuleMsg, relationType string, err error) {
				if err != nil {
					if _, ok := err.(*net.OpError); ok {
						// skip test
					} else {
						t.Fatal("bad", err.Error())
					}
				} else {
					var u = testUser{}
					_ = json.Unmarshal([]byte(msg.GetData()), &u)
					//assert.Equal(t, int64(1), u.Id)
					//assert.Equal(t, 21, u.Age)
					assert.Equal(t, "test01", u.Name)
				}
			},
		},
		{
			Node:    node5_2,
			MsgList: msgList,
			Callback: func(msg types.RuleMsg, relationType string, err error) {
				if err != nil {
					if _, ok := err.(*net.OpError); ok {
						// skip test
					} else {
						t.Fatal("bad", err.Error())
					}
				} else {
					var u = testUser{}
					_ = json.Unmarshal([]byte(msg.GetData()), &u)
					assert.Equal(t, "test01", u.Name)
				}
			},
		},
		{
			Node:    node6,
			MsgList: msgList,
			Callback: func(msg types.RuleMsg, relationType string, err error) {
				if err != nil {
					if _, ok := err.(*net.OpError); ok {
						// skip test
					} else {
						t.Fatal("bad", err.Error())
					}
				} else {
					assert.Equal(t, "1", msg.Metadata.GetValue(rowsAffectedKey))
				}
			},
		},
	}
	for _, item := range nodeList {
		test.NodeOnMsgWithChildren(t, item.Node, item.MsgList, item.ChildrenNodes, item.Callback)
		time.Sleep(time.Millisecond * 20)
	}
	time.Sleep(time.Millisecond * 200)
}

func testConcurrency(t *testing.T, targetNodeType, driverName, dsn string) {

	node, _ := test.CreateAndInitNode(targetNodeType, types.Configuration{
		"driverName": "mysql",
		"dsn":        "ref://local_mysql_client_1",
		"params": []string{
			"${msg.customer_id}",
			"${msg.station_id}",
			"${msg.cabinet_id}",
			"${msg.SubCode}",
			"${msg.PcsCode}",
			"${msg.DirectVol}",
			"${msg.DirectCur}",
			"${msg.DirectActPower}",
			"${msg.DirectReactPower}",
			"${msg.DeviceTemp}",
			"${msg.ABLineVol}",
			"${msg.BCLineVol}",
			"${msg.CALineVol}",
			"${msg.AGridCur}",
			"${msg.BGridCur}",
			"${msg.CGridCur}",
			"${msg.AGridActPower}",
			"${msg.BGridActPower}",
			"${msg.CGridActPower}",
			"${msg.AGridAppPower}",
			"${msg.BGridAppPower}",
			"${msg.CGridAppPower}",
			"${msg.AGridNoactPower}",
			"${msg.BGridNoactPower}",
			"${msg.CGridNoactPower}",
			"${msg.GridFrequency}",
			"${msg.TotActPower}",
			"${msg.TotNoactPower}",
			"${msg.TotAppPower}",
			"${msg.APhasePowerFactor}",
			"${msg.BPhasePowerFactor}",
			"${msg.CPhasePowerFactor}",
			"${msg.GridPowerFac}",
			"${msg.APhaseFreq}",
			"${msg.BPhaseFreq}",
			"${msg.CPhaseFreq}",
			"${msg.TotalFreq}",
			"${msg.RatedVol}",
			"${msg.RatedCur}",
			"${msg.RatedFreq}",
			"${msg.PcsMaxChgActPower}",
			"${msg.PcsMaxDisChgReactPower}",
			"${msg.PcsMaxDisChgVol}",
			"${msg.PcsMaxDisChgCur}",
			"${msg.CommSt}",
			"${msg.RunSt}",
			"${msg.FaultSt}",
			"${msg.StopSt}",
			"${msg.PlcState}",
			"${msg.PlcStateReq}",
			"${msg.PlcIo}",
			"${msg.OnOffGridSt}",
			"${msg.AlarmSt}",
			"${msg.SwitchSt}",
			"${msg.RunMode}",
			"${msg.PowerFactor}",
			"${msg.NoactPowerMode}",
			"${msg.WatchDog}",
			"${msg.PcsState}",
			"${msg.PcsFault}",
			"${msg.FltCode7}",
			"${msg.FltCode8}",
			"${msg.FltCode9}",
			"${msg.FltCode10}",
			"${msg.FltCode11}",
			"${msg.ALineVol}",
			"${msg.BLineVol}",
			"${msg.CLineVol}",
			"${msg.TotPF}",
			"${msg.BackPowerGridEQ}",
			"${msg.PowerGridEQ}",
			"${msg.DeviceTemp}",
			"${msg.ActPowerDispatch}",
			"${msg.DirectChargeEQ}",
			"${msg.DirectDischargeEQ}",
			"${msg.GridTied}",
			"${msg.OffGrid}",
			"${msg.AllPcsCrntSum}",
			"${msg.AllPcsCrntSum}",
			"${msg.AllPcsVolAvg}",
		},
		"poolSize": 30,
		"sql":      "INSERT INTO cn_pcs_real  (customer_id,\n station_id,\n cabinet_id,\n sub_code,\n pcs_code,\n direct_vol,\n direct_cur,\n direct_act_power,\n direct_react_power,\n device_temp,\n ab_line_vol,\n bc_line_vol,\n ca_line_vol,\n a_grid_cur,\n b_grid_cur,\n c_grid_cur,\n a_grid_act_power,\n b_grid_act_power,\n c_grid_act_power,\n a_grid_app_power,\n b_grid_app_power,\n c_grid_app_power,\n a_grid_noact_power,\n b_grid_noact_power,\n c_grid_noact_power,\n grid_frequency,\n tot_act_power,\n tot_noact_power,\n tot_app_power,\n a_phase_power_factor,\n b_phase_power_factor,\n c_phase_power_factor,\n grid_power_fac,\n a_phase_freq,\n b_phase_freq,\n c_phase_freq,\n total_freq,\n rated_vol,\n rated_cur,\n rated_freq,\n pcs_max_chg_act_power,\n pcs_max_dis_chg_react_power,\n pcs_max_dis_chg_vol,\n pcs_max_dis_chg_cur,\n comm_st,\n run_st,\n fault_st,\n stop_st,\n plc_state,\n plc_state_req,\n plc_io,\n on_off_grid_st,\n alarm_st,\n switch_st,\n run_mode,\n refresh_at,\n power_factor,\n noact_power_mode,\n watch_dog,\n pcs_state,\n pcs_fault,\n flt_code_7,\n flt_code_8,\n flt_code_9,\n flt_code_10,\n flt_code_11,\n a_line_vol,\n b_line_vol,\n c_line_vol,\n tot_pf,\n back_power_grid_eq,\n power_grid_eq,\n device_max_temp,\n act_power_dispatch,\n direct_charge_eq,\n direct_discharge_eq,\n grid_tied,\n off_grid,\n all_pcs_crnt_sum,\n all_pcs_power_sum,\n all_pcs_vol_avg) VALUES  (?,\n ?,\n ?,\n ?,\n ?,\n ?,\n ?,\n ?,\n ?,\n ?,\n ?,\n ?,\n ?,\n ?,\n ?,\n ?,\n ?,\n ?,\n ?,\n ?,\n ?,\n ?,\n ?,\n ?,\n ?,\n ?,\n ?,\n ?,\n ?,\n ?,\n ?,\n ?,\n ?,\n ?,\n ?,\n ?,\n ?,\n ?,\n ?,\n ?,\n ?,\n ?,\n ?,\n ?,\n ?,\n ?,\n ?,\n ?,\n ?,\n ?,\n ?,\n ?,\n ?,\n ?,\n ?,\n sysdate(),\n ?,\n ?,\n ?,\n ?,\n ?,\n ?,\n ?,\n ?,\n ?,\n ?,\n ?,\n ?,\n ?,\n ?,\n ?,\n ?,\n ?,\n ?,\n ?,\n ?, \n ?,\n ?,\n ?,\n ?,\n ?) ON DUPLICATE KEY UPDATE  direct_vol = CASE WHEN VALUES(direct_vol) IS NOT NULL THEN VALUES(direct_vol) ELSE direct_vol END,\n direct_cur = CASE WHEN VALUES(direct_cur) IS NOT NULL  THEN VALUES(direct_cur) ELSE direct_cur END,\n direct_act_power = CASE WHEN VALUES(direct_act_power) IS NOT NULL  THEN VALUES(direct_act_power) ELSE direct_act_power END,\n direct_react_power = CASE WHEN VALUES(direct_react_power) IS NOT NULL  THEN VALUES(direct_react_power) ELSE direct_react_power END,\n device_temp = CASE WHEN VALUES(device_temp) IS NOT NULL  THEN VALUES(device_temp) ELSE device_temp END,\n ab_line_vol = CASE WHEN VALUES(ab_line_vol) IS NOT NULL THEN VALUES(ab_line_vol) ELSE ab_line_vol END,\n bc_line_vol = CASE WHEN VALUES(bc_line_vol) IS NOT NULL  THEN VALUES(bc_line_vol) ELSE bc_line_vol END,\n ca_line_vol = CASE WHEN VALUES(ca_line_vol) IS NOT NULL  THEN VALUES(ca_line_vol) ELSE ca_line_vol END,\n a_grid_cur = CASE WHEN VALUES(a_grid_cur) IS NOT NULL  THEN VALUES(a_grid_cur) ELSE a_grid_cur END,\n b_grid_cur = CASE WHEN VALUES(b_grid_cur) IS NOT NULL  THEN VALUES(b_grid_cur) ELSE b_grid_cur END,\n c_grid_cur = CASE WHEN VALUES(c_grid_cur) IS NOT NULL  THEN VALUES(c_grid_cur) ELSE c_grid_cur END,\n a_grid_act_power = CASE WHEN VALUES(a_grid_act_power) IS NOT NULL  THEN VALUES(a_grid_act_power) ELSE a_grid_act_power END,\n b_grid_act_power = CASE WHEN VALUES(b_grid_act_power) IS NOT NULL  THEN VALUES(b_grid_act_power) ELSE b_grid_act_power END,\n c_grid_act_power = CASE WHEN VALUES(c_grid_act_power) IS NOT NULL  THEN VALUES(c_grid_act_power) ELSE c_grid_act_power END,\n a_grid_app_power = CASE WHEN VALUES(a_grid_app_power) IS NOT NULL  THEN VALUES(a_grid_app_power) ELSE a_grid_app_power END,\n b_grid_app_power = CASE WHEN VALUES(b_grid_app_power) IS NOT NULL  THEN VALUES(b_grid_app_power) ELSE b_grid_app_power END,\n c_grid_app_power = CASE WHEN VALUES(c_grid_app_power) IS NOT NULL  THEN VALUES(c_grid_app_power) ELSE c_grid_app_power END,\n a_grid_noact_power = CASE WHEN VALUES(a_grid_noact_power) IS NOT NULL  THEN VALUES(a_grid_noact_power) ELSE a_grid_noact_power END,\n b_grid_noact_power = CASE WHEN VALUES(b_grid_noact_power) IS NOT NULL  THEN VALUES(b_grid_noact_power) ELSE b_grid_noact_power END,\n c_grid_noact_power = CASE WHEN VALUES(c_grid_noact_power) IS NOT NULL  THEN VALUES(c_grid_noact_power) ELSE c_grid_noact_power END,\n grid_frequency = CASE WHEN VALUES(grid_frequency) IS NOT NULL  THEN VALUES(grid_frequency) ELSE grid_frequency END,\n tot_act_power = CASE WHEN VALUES(tot_act_power) IS NOT NULL THEN VALUES(tot_act_power) ELSE tot_act_power END,\n tot_noact_power = CASE WHEN VALUES(tot_noact_power) IS NOT NULL  THEN VALUES(tot_noact_power) ELSE tot_noact_power END,\n tot_app_power = CASE WHEN VALUES(tot_app_power) IS NOT NULL  THEN VALUES(tot_app_power) ELSE tot_app_power END,\n a_phase_power_factor = CASE WHEN VALUES(a_phase_power_factor) IS NOT NULL  THEN VALUES(a_phase_power_factor) ELSE a_phase_power_factor END,\n b_phase_power_factor = CASE WHEN VALUES(b_phase_power_factor) IS NOT NULL  THEN VALUES(b_phase_power_factor) ELSE b_phase_power_factor END,\n c_phase_power_factor = CASE WHEN VALUES(c_phase_power_factor) IS NOT NULL  THEN VALUES(c_phase_power_factor) ELSE c_phase_power_factor END,\n grid_power_fac = CASE WHEN VALUES(grid_power_fac) IS NOT NULL  THEN VALUES(grid_power_fac) ELSE grid_power_fac END,\n a_phase_freq = CASE WHEN VALUES(a_phase_freq) IS NOT NULL  THEN VALUES(a_phase_freq) ELSE a_phase_freq END,\n b_phase_freq = CASE WHEN VALUES(b_phase_freq) IS NOT NULL  THEN VALUES(b_phase_freq) ELSE b_phase_freq END,\n c_phase_freq = CASE WHEN VALUES(c_phase_freq) IS NOT NULL  THEN VALUES(c_phase_freq) ELSE c_phase_freq END,\n total_freq = CASE WHEN VALUES(total_freq) IS NOT NULL  THEN VALUES(total_freq) ELSE total_freq END,\n rated_vol = CASE WHEN VALUES(rated_vol) IS NOT NULL THEN VALUES(rated_vol) ELSE rated_vol END,\n rated_cur = CASE WHEN VALUES(rated_cur) IS NOT NULL  THEN VALUES(rated_cur) ELSE rated_cur END,\n rated_freq = CASE WHEN VALUES(rated_freq) IS NOT NULL  THEN VALUES(rated_freq) ELSE rated_freq END,\n pcs_max_chg_act_power = CASE WHEN VALUES(pcs_max_chg_act_power) IS NOT NULL  THEN VALUES(pcs_max_chg_act_power) ELSE pcs_max_chg_act_power END,\n pcs_max_dis_chg_react_power = CASE WHEN VALUES(pcs_max_dis_chg_react_power) IS NOT NULL  THEN VALUES(pcs_max_dis_chg_react_power) ELSE pcs_max_dis_chg_react_power END,\n pcs_max_dis_chg_vol = CASE WHEN VALUES(pcs_max_dis_chg_vol) IS NOT NULL  THEN VALUES(pcs_max_dis_chg_vol) ELSE pcs_max_dis_chg_vol END,\n pcs_max_dis_chg_cur = CASE WHEN VALUES(pcs_max_dis_chg_cur) IS NOT NULL  THEN VALUES(pcs_max_dis_chg_cur) ELSE pcs_max_dis_chg_cur END,\n comm_st = VALUES(comm_st),\n run_st = VALUES(run_st),\n fault_st = VALUES(fault_st),\n stop_st = VALUES(stop_st),\n plc_state = VALUES(plc_state),\n plc_state_req = VALUES(plc_state_req),\n plc_io = VALUES(plc_io),\n on_off_grid_st = VALUES(on_off_grid_st),\n alarm_st = VALUES(alarm_st),\n switch_st = VALUES(switch_st),\n run_mode = VALUES(run_mode),\nrefresh_at = VALUES(refresh_at),\npower_factor= CASE WHEN VALUES(power_factor ) IS NOT NULL  THEN VALUES(power_factor ) ELSE power_factor END,\nnoact_power_mode= CASE WHEN VALUES(noact_power_mode ) IS NOT NULL  THEN VALUES(noact_power_mode ) ELSE noact_power_mode END,\nwatch_dog = CASE WHEN VALUES(watch_dog) IS NOT NULL AND VALUES(watch_dog) != 0 THEN VALUES(watch_dog) ELSE watch_dog END,\npcs_state = CASE WHEN VALUES(pcs_state) IS NOT NULL  THEN VALUES(pcs_state) ELSE pcs_state END,\npcs_fault = CASE WHEN VALUES(pcs_fault) IS NOT NULL  THEN VALUES(pcs_fault) ELSE pcs_fault END,\nflt_code_7= CASE WHEN VALUES(flt_code_7 ) IS NOT NULL THEN VALUES(flt_code_7 ) ELSE flt_code_7 END,\nflt_code_8= CASE WHEN VALUES(flt_code_8 ) IS NOT NULL THEN VALUES(flt_code_8 ) ELSE flt_code_8 END,\nflt_code_9= CASE WHEN VALUES(flt_code_9 ) IS NOT NULL THEN VALUES(flt_code_9 ) ELSE flt_code_9 END,\nflt_code_10 = CASE WHEN VALUES(flt_code_10) IS NOT NULL THEN VALUES(flt_code_10) ELSE flt_code_10 END,\nflt_code_11 = CASE WHEN VALUES(flt_code_11) IS NOT NULL THEN VALUES(flt_code_11) ELSE flt_code_11 END,\na_line_vol= CASE WHEN VALUES(a_line_vol ) IS NOT NULL  THEN VALUES(a_line_vol ) ELSE a_line_vol END,\nb_line_vol= CASE WHEN VALUES(b_line_vol ) IS NOT NULL  THEN VALUES(b_line_vol ) ELSE b_line_vol END,\nc_line_vol= CASE WHEN VALUES(c_line_vol ) IS NOT NULL  THEN VALUES(c_line_vol ) ELSE c_line_vol END,\ntot_pf= CASE WHEN VALUES(tot_pf ) IS NOT NULL  THEN VALUES(tot_pf ) ELSE tot_pf END,\nback_power_grid_eq= CASE WHEN VALUES(back_power_grid_eq ) IS NOT NULL  THEN VALUES(back_power_grid_eq ) ELSE back_power_grid_eq END,\npower_grid_eq = CASE WHEN VALUES(power_grid_eq) IS NOT NULL  THEN VALUES(power_grid_eq) ELSE power_grid_eq END,\ndevice_max_temp = CASE WHEN VALUES(device_max_temp) IS NOT NULL  THEN VALUES(device_max_temp) ELSE device_max_temp END,\nact_power_dispatch= CASE WHEN VALUES(act_power_dispatch ) IS NOT NULL  THEN VALUES(act_power_dispatch ) ELSE act_power_dispatch END,\ndirect_charge_eq= CASE WHEN VALUES(direct_charge_eq ) IS NOT NULL  THEN VALUES(direct_charge_eq ) ELSE direct_charge_eq END,\ndirect_discharge_eq = CASE WHEN VALUES(direct_discharge_eq) IS NOT NULL  THEN VALUES(direct_discharge_eq) ELSE direct_discharge_eq END,\ngrid_tied = CASE WHEN VALUES(grid_tied) IS NOT NULL  THEN VALUES(grid_tied) ELSE grid_tied END,\noff_grid= CASE WHEN VALUES(off_grid ) IS NOT NULL THEN VALUES(off_grid ) ELSE off_grid END,\nall_pcs_crnt_sum= CASE WHEN VALUES(all_pcs_crnt_sum ) IS NOT NULL THEN VALUES(all_pcs_crnt_sum ) ELSE all_pcs_crnt_sum END,\nall_pcs_power_sum= CASE WHEN VALUES(all_pcs_power_sum ) IS NOT NULL THEN VALUES(all_pcs_power_sum ) ELSE all_pcs_power_sum END,\nall_pcs_vol_avg= CASE WHEN VALUES(all_pcs_vol_avg ) IS NOT NULL THEN VALUES(all_pcs_vol_avg ) ELSE all_pcs_vol_avg END;",
	}, Registry)

	var msgStr = `
{
  "ABLineVol": 0,
  "AGridActPower": 12.3,
  "AGridAppPower": 12.3,
  "AGridCur": 55.1,
  "AGridNoactPower": -0.5,
  "ALineVol": 227.4,
  "APhaseFreq": 0,
  "APhasePowerFactor": 1,
  "ActPowerDispatch": 0,
  "AlarmSt": 0,
  "AllPcsCrntSum": 55.03333333333334,
  "AllPcsPowerSum": 37.1,
  "AllPcsVolAvg": 227.13333333333335,
  "BCLineVol": 0,
  "BGridActPower": 12.3,
  "BGridAppPower": 12.3,
  "BGridCur": 55.6,
  "BGridNoactPower": -0.4,
  "BLineVol": 225.2,
  "BPhaseFreq": 0,
  "BPhasePowerFactor": 1,
  "BackPowerGridEQ": 0,
  "CALineVol": 0,
  "CGridActPower": 12.2,
  "CGridAppPower": 12.2,
  "CGridCur": 54.400000000000006,
  "CGridNoactPower": -0.5,
  "CLineVol": 228.8,
  "CPhaseFreq": 0,
  "CPhasePowerFactor": 1,
  "CommSt": 0,
  "DeviceTemp": 40,
  "DirectActPower": 38.8,
  "DirectChargeEQ": 0,
  "DirectCur": 45.900000000000006,
  "DirectDischargeEQ": 0,
  "DirectReactPower": 0,
  "DirectVol": 848.1,
  "FaultSt": 0,
  "FltCode10": 0,
  "FltCode11": 0,
  "FltCode7": 0,
  "FltCode8": 0,
  "FltCode9": 0,
  "GridFrequency": 49.99,
  "GridPowerFac": 0,
  "GridTied": 0,
  "NoactPowerMode": 0,
  "OffGrid": 0,
  "OnOffGridSt": 0,
  "PcsCode": "Pcs1",
  "PcsFault": 0,
  "PcsMaxChgActPower": 0,
  "PcsMaxDisChgCur": 0,
  "PcsMaxDisChgReactPower": 0,
  "PcsMaxDisChgVol": 0,
  "PcsState": 0,
  "PlcIo": 0,
  "PlcState": 0,
  "PlcStateReq": 0,
  "PowerFactor": 0,
  "PowerGridEQ": 0,
  "RatedCur": 0,
  "RatedFreq": 0,
  "RatedVol": 0,
  "RunMode": 0,
  "RunSt": 0,
  "StkCode": "Stk1",
  "StopSt": 0,
  "SubCode": "",
  "SwitchSt": 0,
  "Time": 1746009114192,
  "TotActPower": 37.1,
  "TotAppPower": 36.8,
  "TotNoactPower": -1.4,
  "TotPF": 1,
  "TotalFreq": 0,
  "WatchDog": 0,
  "cabinet_id": 48,
  "customer_id": 66,
  "id": "8c32232cc41c:2",
  "station_id": 169
}
`

	metaData := types.BuildMetadata(make(map[string]string))
	metaData.PutValue("id", "1")
	metaData.PutValue("name", "test01")
	metaData.PutValue("age", "18")
	metaData.PutValue("sql", "select * from users where id = ?")

	msgList := []test.Msg{
		{
			DataType: types.JSON,
			MetaData: metaData,
			MsgType:  "ACTIVITY_EVENT2",
			Data:     msgStr,
		},
	}

	var nodeList = []test.NodeAndCallback{
		{
			Node:    node,
			MsgList: msgList,
			Callback: func(msg types.RuleMsg, relationType string, err error) {
				assert.Equal(t, types.Failure, relationType)
			},
		},
	}
	var i = 0
	for _, item := range nodeList {
		for i < 1000 {
			test.NodeOnMsgWithChildren(t, item.Node, item.MsgList, item.ChildrenNodes, item.Callback)
			i++
		}
	}
	time.Sleep(time.Millisecond * 5000)
}

type testUser struct {
	Id   int64  `json:"id"`
	Name string `json:"name"`
	Age  int    `json:"age"`
}
