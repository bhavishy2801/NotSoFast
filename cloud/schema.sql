-- Run once in the Supabase SQL editor. Client apps use only a PUBLIC key.
create table if not exists public.nsf_snapshots (
 id uuid primary key default gen_random_uuid(),
 user_id uuid not null references auth.users(id) on delete cascade,
 created_at timestamptz not null default now(),
 payload jsonb not null check (octet_length(payload::text) <= 100000)
);
alter table public.nsf_snapshots enable row level security;
revoke all on public.nsf_snapshots from anon;
grant select, insert, delete on public.nsf_snapshots to authenticated;
create policy "Read own saves" on public.nsf_snapshots for select to authenticated using ((select auth.uid()) = user_id);
create policy "Save own work" on public.nsf_snapshots for insert to authenticated with check ((select auth.uid()) = user_id);
create policy "Delete own saves" on public.nsf_snapshots for delete to authenticated using ((select auth.uid()) = user_id);
create index if not exists nsf_snapshots_user_created on public.nsf_snapshots(user_id, created_at desc);
-- Retain the newest ten saves per account. Invoker security preserves RLS.
create or replace function public.nsf_trim_saves() returns trigger
language plpgsql security invoker set search_path = '' as $$
begin
 delete from public.nsf_snapshots where user_id = new.user_id and id in
 (select id from public.nsf_snapshots where user_id = new.user_id order by created_at desc, id desc offset 10);
 return new;
end;
$$;
create trigger nsf_trim_saves after insert on public.nsf_snapshots for each row execute function public.nsf_trim_saves();
