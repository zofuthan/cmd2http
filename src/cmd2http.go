package main


import (
	"fmt"
	"log"
	 "net/http"
	 "flag"
	 "os"
	 "bytes"
	 "time"
	 "strings"
	 "regexp"
	 "encoding/json"
	 "os/exec"
	 "text/template"
	 jsonConf "github.com/daviddengcn/go-ljson-conf"
   )
var configPath=flag.String("conf","./cmd2http.conf","config file")

var port int

const VERSION="20131031 1.0"

type param struct{
  name string
  defaultValue string
  isValParam bool
}

type Conf struct{
   name string
   cmdStr string
   cmd string
   charset string
   params []*param
}

var confMap map[string]*Conf

var config *jsonConf.Conf

func main(){
   flag.Parse()
   logFile,_:=os.OpenFile("./cmd2http.log",os.O_CREATE|os.O_RDWR|os.O_APPEND,0666)
   defer logFile.Close()
   log.SetOutput(logFile)
//    log.SetFlags(log.LstdFlags|log.Lshortfile)
    loadConfig()
    
    startHttpServer()
}

func (p *param)ToString() string{
    return fmt.Sprintf("name:%s,default:%s,isValParam:%x",p.name,p.defaultValue,p.isValParam);
}
func loadConfig(){
   log.Println("start load conf [",*configPath,"]")
   var err error
   log.Println("use conf:",*configPath)
   config, err= jsonConf.Load(*configPath)
	if err != nil {
	  log.Println(err.Error(),config)
	  os.Exit(2)
	}
	port=config.Int("port",8310)
	
	confMap=make(map[string]*Conf)
	
	cmds:=config.Object("cmds",make(map[string]interface{}))
	
	for k,v:=range cmds{
	   _conf:=v.(map[string]interface{})
	   conf:=new(Conf)
	   conf.name=k
	   conf.charset="utf-8"
	   charset,has:=_conf["charset"]
	   if(has){
	      conf.charset=charset.(string)
	    }
	   conf.cmdStr,_=_conf["cmd"].(string)
	   conf.cmdStr=strings.TrimSpace(conf.cmdStr)
	   conf.params=make([]*param,0,10)
	   
	   ps:=regexp.MustCompile(`\s+`).Split(conf.cmdStr,-1)
//	   fmt.Println(ps)
	   conf.cmd=ps[0]
	   
	   for i:=1;i<len(ps);i++ {
	       item:=ps[i]
//	       fmt.Println("i:",i,item)
	        _param:=new(param)
	        _param.name=item
	       
	       if(item[0]=='$'){
	        _param.isValParam=true;
	        tmp:=strings.Split(item+"|","|")
	        _param.name=tmp[0][1:]
	        _param.defaultValue=tmp[1]
	       }
	       conf.params=append(conf.params,_param)
//	       fmt.Println(_param.name,_param.defaultValue)
	    }
	   log.Println("register[",k,"] cmd:",conf.cmdStr)
	   confMap[k]=conf
	}
	
	log.Println("load conf [",*configPath,"] finish [ok]")
}

func startHttpServer(){
//   http.ReadTimeout=60 * time.Second
   
   http.Handle("/s/",http.FileServer(http.Dir("./")))
   http.HandleFunc("/",myHandler_root)
   http.HandleFunc("/help",myHandler_help)
   
   addr:=fmt.Sprintf(":%d",port)
   log.Println("listen at",addr)
   fmt.Println("listen at",addr)
   
   http.ListenAndServe(addr,nil)
}

func Command(name string, args []string) *exec.Cmd {
	aname, err := exec.LookPath(name)
	if err != nil {
		aname = name
	}
	return &exec.Cmd{
		Path: aname,
		Args: args,
	}
}


func myHandler_root(w http.ResponseWriter, r *http.Request){
     startTime:=time.Now()
	  path:=strings.Trim(r.URL.Path,"/")
	  if(path==""){
	       _,err := os.Stat( "./s/index.html" )
			if err == nil {
		     http.Redirect(w,r,"/s/",302)
			  return;
			}
	      myHandler_help(w,r)
	      return;
	   }
	   
	   
	  logStr:=r.RemoteAddr+" req:"+r.RequestURI
	  defer func(){
	       log.Println(logStr)
	   }()
	  
	  conf,has:=confMap[path]
	  if(!has) {
	     logStr=logStr+" not support cmd"
	     fmt.Fprintf(w,"not support")
	     return;
	  }
	  
	  args:=make([]string,len(conf.params)+1)
	  for i,_param:=range conf.params{
		  if(!_param.isValParam){
	        args[i+1]=_param.name
		     continue
		  }
	     val:=r.FormValue(_param.name)
	     if(val==""){
	        val=_param.defaultValue
	      }
	      args[i+1]=val
	  }
	  cmd := Command(conf.cmd,args)
	  var out bytes.Buffer
		cmd.Stdout = &out
	  err := cmd.Run()
	  if err != nil {
	    log.Println(err)
	    fmt.Fprintf(w,err.Error())
	    return;
	  }
	  format:=r.FormValue("format")
	  str:=`<!DOCTYPE html><html><head>
	         <meta http-equiv='Content-Type' content='text/html; charset=%s' />
	         <title>%s cmd2http</title></head><body><pre>%s</pre></body></html>`
	         
	  outStr:=out.String()
	  logStr=logStr+fmt.Sprintf(" resLen:%d time_use:%v",len(outStr),time.Now().Sub(startTime))
	  
	  if(format=="" || format=="html"){
	    fmt.Fprintf(w,fmt.Sprintf(str,conf.charset,conf.name,outStr))
	   }else if(format=="jsonp"){
	       cb:=r.FormValue("cb")
	       if(cb==""){
	           cb="cb"
	        }
	       m:=make(map[string]string)
	       m["data"]=outStr
	       jsonByte,_:=json.Marshal(m)
	       fmt.Fprintf(w,fmt.Sprintf(`%s(%s)`,cb,string(jsonByte)))
	   }else{ 
	    fmt.Fprintf(w,outStr)
	   }
}


func myHandler_help(w http.ResponseWriter, r *http.Request){
   str:=`<!DOCTYPE html><html>
         <head>
         <meta http-equiv='Content-Type' content='text/html; charset=utf-8' />
         <title>{{.title}} cmd2http {{.version}}</title>
         <style>
				.cpanel{background:#ffffff;border-radius:10px;margin-bottom: 10px;border:1px solid #e6eaed;}
				.cpanel .hd{background:#e6eaed;padding: 3px 0 3px 10px;border-radius:10px 10px 0 0;color:#000;font-weight: bold;}
				.cpanel .bd{padding: 10px;font-size:13px}
				h1,p{margin:5px 0 5px}
         </style>
          <script>
            function $(id){
              return document.getElementById(id);
               }
           function jsonp(url){
           var script = document.createElement('script');
               script.setAttribute('src', url+"&format=jsonp&cb=callback");
               document.getElementsByTagName('head')[0].appendChild(script); 
               }
               
           function callback(data){
                var doc=window.frames[0].document;
                doc.open();
					 doc.close();
					 doc.body.innerHTML="<pre>"+data.data+"</pre>";
               }
          function form_check(){
               var cmd=$('cmd').value;
               if(!cmd){
                    alert("pls choose cmd");
                    return false;
                    }
                var _param=$('cmd').value+"?"+$('params').value;
                var _url="http://"+location.host+"/"+_param;
                $('div_url').innerHTML="<a href='"+_url+"' target='_blank'>"+_url+"</a>";
                $('panel_result').style.display="block";
                $('result').src=_param;
               /* jsonp(_param)*/
             }
          function cmd_change(){
                $('msg').innerHTML="<br/>command defined : <b>"+(msg[$('cmd').value]||"")+"</b>";
             }
          </script>
        </head><body>
          <h1>{{.title}}<font style='font-size:16px'>&nbsp;cmd2http</font></h1>
          <div style='margin-bottom:5px'>{{.intro}}</div>
          <div class="cpanel">
             <div class="hd">demo</div>
             <div class='bd'>
		          <p>defined cmd: <b>echo -n $wd $a $b|defaultValue</b> </p>
		          <p>http://localhost/<b>echo?wd=hello&a=world</b>
		             ==&gt;   <b>echo -n hello world defaultValue</b> 
		          </p>
		           <p>support output format with param : &format=[""|html|jsonp|txt]</p>
	          </div>
          </div>
          <br/>
           <div class="cpanel">
             <div class="hd">quick cmd</div>
             <div class='bd'>
             
          <form onsubmit='form_check();return false;'>
          cmd:<select id='cmd' onchange='cmd_change()' autocomplete="off">
            <option value=''>pls choose cmd</option>
              {{.option_cmd}}
           </select>
               params:<input type='text' id='params' name='params' style="width:500px">
               <input type='submit'>
             <div id='msg'></div>
          </form>
          <br/>
          </div>
          </div>
          <script> 
          var msg={{.msgs}};
	         function ifr_load(){
	            $("result").height=50;
				   $("result").height=window.frames[0].document.body.scrollHeight+40;
	            }
          </script>
           <div class="cpanel" style="display:none" id='panel_result'>
           <div class="hd">result &nbsp;<span id="div_url"></span></div>
             <div class='bd'>
               <iframe id='result' name="result" src="about:_blank" style="border:none;width:99%" onload="ifr_load()" ></iframe>
            </div>
          </div>
          </body></html>`;
        
       msgs:=make(map[string]string)
       option_cmd:=""
       for name,_conf:=range confMap{
           option_cmd=option_cmd+"<option value="+name+">"+name+"</option>";
           msgs[name]=_conf.cmdStr
          }
        
	   title:=config.String("title","")
	   str=regexp.MustCompile(`\s+`).ReplaceAllString(str," ")
	   
	   tpl,_:=template.New("page").Parse(str)
	   values :=make(map[string]string)
	   values["version"]=VERSION
	   values["title"]=title
	   values["intro"]=config.String("intro","")
	   values["option_cmd"]=option_cmd
	   
	   jsonByte,_:=json.Marshal(msgs)
	   values["msgs"]=string(jsonByte)
	   
	   w.Header().Add("c2h",VERSION)
	   tpl.Execute(w,values)
}
