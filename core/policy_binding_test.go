package core

import("context";"testing")

func TestCannotSelectWeakerPolicy(t *testing.T){
 s,_,head:=fixture(t);ctx:=context.Background();s.cfg.Policies["weaker"]=Policy{Version:"1",Kind:"literal_bytes",Marker:[]byte("new marker"),MaxBytes:1024}
 r:=searchTest(t,s,head,Predicate{"literal_bytes",[]byte("new marker"),1},Scope{})
 q:=CreateRequest{Repository:"demo",Snapshot:head,Operation:"bypass",Policy:"weaker",PolicyVersion:"1",Path:"database.yaml",Content:[]byte("new marker"),Receipts:[]string{r.ID}}
 if _,e:=s.GuardedCreate(ctx,"alice",q);e==nil{t.Fatal("weaker policy bypassed basename convention")}
}
