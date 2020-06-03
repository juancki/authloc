
package dbutils

import (
	"fmt"
	"strings"
	"time"
)


type EarthquakeElement struct {
    User string
    Location string
    Time time.Time
    Longitude float64
    Latitude float64
}

func (rclient *Redis) DeleteEarhquake(msgids ...string) error{
    keys := make([]string, len(msgids))
    for ind, value := range msgids {
        keys[ind] = MSGQUAKE + value
    }
    rclient.Lock()
    err := rclient.Redis.Del(keys...).Err()
    rclient.Unlock()
    if err != nil{
        return err
    }
    return nil
}

func (rclient *Redis) AddEarhquakeToMsg(msgid, user, loc string) error{
    now := time.Now().Format(time.RFC3339)
    value := fmt.Sprintf("%s,%s,%s", user, loc, now) // Comma separated values
    key := MSGQUAKE + msgid
    rclient.Lock()
    err := rclient.Redis.LPush(key,value).Err()
    rclient.Unlock()
    if err != nil{
        return err
    }
    return nil
}

func (rclient *Redis) EarhquakeCount(msgid, user, loc string) (int64, error) {
    key := MSGQUAKE + msgid
    rclient.Lock()
    res := rclient.Redis.LLen(key)
    rclient.Unlock()
    err := res.Err()
    if err != nil{
        return 0, err
    }
    return res.Val(), nil
}

func (rclient *Redis) EarhquakeElements(msgid string, init,fin int64) (<-chan *EarthquakeElement, int, error) {
    // Returnss a channel with the elements from the list
    //          the number of elementss in the list
    //          Any error in the aquisition of the data.
    c := make(chan *EarthquakeElement)
    defer close(c)
    key := MSGQUAKE + msgid
    rclient.Lock()
    res := rclient.Redis.LRange(key,init,fin)
    rclient.Unlock()
    err := res.Err()
    if err != nil{
        return nil, 0, err
    }
    go func(){
        for _,value := range res.Val(){
            splits := strings.Split(value,",") // Comma separated values (user, loc, time)
            long, lat := CoorFromString(splits[1])
            t, _ := time.Parse(time.RFC3339,splits[2])
            element := &EarthquakeElement{
                    User: splits[0],
                    Location: splits[1],
                    Time : t,
                    Longitude: long,
                    Latitude: lat,
                }
            c<- element
        }
    }()
    return c, len(res.Val()), nil
}

