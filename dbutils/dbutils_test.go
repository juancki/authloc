package dbutils

import (
	"testing"
	"time"
)


func TestBase64AndStrings(t *testing.T){
    str := "my basic testing String: 1234124\n"
    b64 := string2base64(str)
    if str != base64_2_string(b64) {
        t.Errorf("Base 64 encoding was incorrect")
    }
}


func TestHourGougeAndTimeWindow(t *testing.T){
    // Basic arithmetic first. Number to string
    if hourGougeToTimeWindow(0) != "AA"{
        t.Errorf("hourGougeToTimeWindow Error 1")
    }
    tw := hourGougeToTimeWindow(24)
    if tw != "BA"{
        t.Errorf("hourGougeToTimeWindow Error 2: Expected `BA` got `%s`.", tw)
    }
    tw = hourGougeToTimeWindow(30*24)
    if tw != "eA"{
        t.Errorf("hourGougeToTimeWindow Error 2: Expected `eA`got `%s`.", tw)
    }
    tw = hourGougeToTimeWindow(23)
    if tw != "AX"{
        t.Errorf("hourGougeToTimeWindow Error 3: Expected `AX`got `%s`.", tw)
    }
    // String to number
    hg := timeWindowToHourGouge("AA")
    if hg != 0 {
        t.Errorf("timeWindowToHourGouge Error 1: Expected `0` got `%d`.", hg)
    }
    hg = timeWindowToHourGouge("AY")
    if hg != -1 {
        t.Errorf("timeWindowToHourGouge Error 2: Expected `-1` got `%d`.", hg)
    }
    hg = timeWindowToHourGouge("AX")
    if hg != 23 {
        t.Errorf("timeWindowToHourGouge Error 3: Expected `23` got `%d`.", hg)
    }
    hg = timeWindowToHourGouge("eA")
    if hg != 24*30 {
        t.Errorf("timeWindowToHourGouge Error 4: Expected `24*30` got `%d`.", hg)
    }
    hg = timeWindowToHourGouge("/A")
    if hg != 24*63 {
        t.Errorf("timeWindowToHourGouge Error 5: Expected `24*63` got `%d`.", hg)
    }
}

func TestTimeAndTimeWindow(t *testing.T){
    // time to string
    tm := Date(1975,1,1)
    tw := getTimeWindow(tm)
    if tw != "AA" {
        t.Errorf("getTimeWindow Error 1: Expected `AA` got `%s`.", tw)
    }
    tm = Date(1975,1,2)
    tw = getTimeWindow(tm)
    if tw != "BA" {
        t.Errorf("getTimeWindow Error 2: Expected `BA` got `%s`.", tw)
    }
    tm = time.Date(1975,1,1,1,0,0,0,time.UTC)
    tw = getTimeWindow(tm)
    if tw != "AB" {
        t.Errorf("getTimeWindow Error 2: Expected `AB` got `%s`.", tw)
    }
    tm = time.Date(1975,1,1,1,59,59,0,time.UTC)
    tw = getTimeWindow(tm)
    if tw != "AB" {
        t.Errorf("getTimeWindow Error 2: Expected `AB` got `%s`.", tw)
    }
    tm = time.Now()
    tw = getTimeWindow(tm)
    tinit, tfin := timeWindowToTime(tw)
    if tm.Sub(tinit) < 0 && tm.Sub(tfin) > 0 {
        // t.Errorf("getTimeWindow Error 2: Expected `/X` got `%s`.", tw)
        t.Error(tm.Format(time.RFC3339), tw, timeWindowToHourGouge(tw))
        t.Error(tinit.Format(time.RFC3339), getTimeWindow(tinit))
        t.Error(tfin.Format(time.RFC3339), getTimeWindow(tfin))
    }

}



