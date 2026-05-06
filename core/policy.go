package core

import("bytes";"context";"path";"sort")

// Applicable conventions are conjunctive: choosing a policy never disables another.
func(s *Service)checkPolicies(ctx context.Context,user string,q CreateRequest)(map[string]Decision,string,error){
 decisions:=map[string]Decision{};policies:=map[string]Policy{};groups:=map[string][]string{}
 snap,e:=s.snapshot(ctx,user,q.Repository,q.Snapshot);if e!=nil{return nil,"",e}
 for _,id:=range q.Receipts{r,e:=s.Receipt(ctx,user,id);if e!=nil{return nil,"",e};if r.Repository!=q.Repository||r.Snapshot!=q.Snapshot{return nil,"",fail("INVALID_EVIDENCE","incompatible receipt")};if e=validateReceipt(r,snap);e!=nil{return nil,"",e};key:=digest(r.Predicate);groups[key]=append(groups[key],id)}
 var names []string;for name,p:=range s.cfg.Policies{if under(q.Path,p.DestinationPrefix){names=append(names,name);policies[name]=p}};sort.Strings(names)
 for _,name:=range names{p:=policies[name];ent:=Entry{Path:q.Path,Type:"regular",Mode:"100644"};if len(q.Content)>p.MaxBytes||!p.Scope.selects(ent){return nil,"",fail("POLICY_DENIED","candidate violates applicable convention")}
  pred:=Predicate{Kind:p.Kind,Version:1,Value:[]byte(path.Base(q.Path))};if p.Kind=="literal_bytes"{pred.Value=p.Marker;if !bytes.Contains(q.Content,p.Marker){return nil,"",fail("POLICY_DENIED","candidate lacks required literal marker")}}
  d,e:=s.Verify(ctx,user,Claim{Repository:q.Repository,Snapshot:q.Snapshot,Predicate:pred,Scope:p.Scope,Receipts:groups[digest(pred)]});if e!=nil{return nil,"",e};if d.Outcome!="SUPPORTED"{return nil,"",fail(d.Outcome,d.Reason)};decisions[name]=d
 };return decisions,digest(policies),nil
}
