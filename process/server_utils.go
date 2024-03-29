package process

import (
	ml "cs425/mp/membershiplist"
	"cs425/mp/util"
	"encoding/json"
	"errors"
	"log"
	"net"
	"reflect"
)

// Define the RPC server struct
type Server struct {
	// This struct is passed as argument, and we made reflection over this interface to call the methods
	caller interface{}
	// If the server get any error handle the information, is represented here
	Error error
	// Represent the addr of the server
	addr string
}

/**
* Create a new instance of *Server struct
 */
func NewServer(i interface{}, addr string) *Server {
	return &Server{caller: i, addr: addr}
}

/**
* Call methods associated to Server.caller using reflection
 */
func (server *Server) callMethod(methodName string, args interface{}) (string, error) {

	var ptr, value, finalMethod reflect.Value

	value = reflect.ValueOf(server.caller)

	// if we start with a pointer, we need to get value pointed to
	// if we start with a value, we need to get a pointer to that value
	if value.Type().Kind() == reflect.Ptr {
		ptr = value
		value = ptr.Elem()
	} else {
		ptr = reflect.New(reflect.TypeOf(server.caller))
		temp := ptr.Elem()
		temp.Set(value)
	}

	// check for method on value
	method := value.MethodByName(methodName)
	if method.IsValid() {
		finalMethod = method
	}
	// check for method on pointer
	method = ptr.MethodByName(methodName)
	if method.IsValid() {
		finalMethod = method
	}

	if finalMethod.IsValid() {
		in := make([]reflect.Value, method.Type().NumIn())

		// Get objects passed as params for the methods
		objects, err := util.InterfaceArrayFromInterface(args)
		if err != nil {
			return "", err
		}
		if len(objects) != method.Type().NumIn() {
			return "", errors.New("differ number of params in call")
		}
		for i := 0; i < method.Type().NumIn(); i++ {
			typeOfObj := reflect.TypeOf(objects[i]).Name()
			typeOfMethodParams := method.Type().In(i).Name()
			if typeOfObj != typeOfMethodParams {
				return "", errors.New("differ params type")
			}
			object := objects[i]
			in[i] = reflect.ValueOf(object)
		}
		response := finalMethod.Call(in)
		if len(response) == 0 {
			return "", nil
		}
		switch v := response[0]; v.Kind() {
		case reflect.String:
			return v.String(), nil
		default:
			return "", errors.New("")
		}
	}

	// return or panic, method not found of either type
	return "", errors.New("error calling method")
}

/**
* Handle the rpc call of methods
 */
func (server *Server) handleOptions(pc net.PacketConn, addr net.Addr, buf []byte, n int, memberList *ml.MembershipList) {
	var requestRPCCall util.RPCBase
	err := json.Unmarshal(buf[:n], &requestRPCCall)
	if err != nil {
		log.Printf("Error unMarshalling")
	}

	// Handle rpc options
	args := make([]interface{}, 0)
	var serialisedMemberList []byte

	if memberList == nil {
		serialisedMemberList, _ = json.Marshal(GetMemberList().GetList())

	} else {
		serialisedMemberList, _ = json.Marshal(memberList.GetList())

	}

	args = append(args, string(serialisedMemberList))
	requestRPCCall.Args = args
	response, err := server.callMethod(requestRPCCall.MethodName, requestRPCCall.Args)
	if err != nil {
		log.Printf("Couldn't call method " + err.Error())
		return
	}

	str, err := json.Marshal(&util.ResponseRPC{Response: response, Error: nil})
	if err != nil {
		_, err = pc.WriteTo(str, addr)
		log.Printf("Couldn't marshal response for rpc " + err.Error())
		return
	}

	_, err = pc.WriteTo(str, addr)
	if err != nil {
		log.Printf("Couldn't send buffer " + err.Error())
		return
	}

}

/**
* Listen in server.addr for calls of the rpc
 */
func (server *Server) ListenServer(exit chan bool, memberList *ml.MembershipList) {
	pc, err := net.ListenPacket("udp", server.addr)
	if err != nil {
		log.Printf("Error Listening to packet\n%v\n", err)
	}
	defer pc.Close()

	for {
		buf := make([]byte, 4096)
		n, addr, err := pc.ReadFrom(buf)
		if err != nil {
			server.Error = err
			break
		}
		go server.handleOptions(pc, addr, buf, n, memberList)

	}
	exit <- true
}
