create table public.users
(
    id       serial
        constraint users_pk
            primary key,
    username text not null
        constraint users_pk_2
            unique,
    hash     text not null
);

alter table public.users
    owner to orchestrator;

create table public.expressions
(
    expressionid uuid not null
        constraint expressions_pk
            primary key,
    expression   text not null,
    status       integer,
    result       double precision,
    userid       integer
        constraint expressions_users_id_fk
            references public.users
);

comment on column public.expressions.expressionid is 'UUID запроса';

comment on column public.expressions.expression is 'Выражение';

comment on column public.expressions.status is 'Статус выражения';

comment on column public.expressions.result is 'Результат вычислений';

alter table public.expressions
    owner to orchestrator;

create table public.operations
(
    operationid  uuid not null
        constraint operationid_pk
            primary key,
    operator     text not null,
    v1           double precision,
    v2           double precision,
    expressionid uuid not null
        constraint expressionid_fk
            references public.expressions,
    parentid     uuid,
    "left"       boolean,
    result       double precision,
    status       integer,
    changedtime  timestamp
);

comment on column public.operations.operationid is 'UUID элементарного выражения';

comment on column public.operations.v1 is 'Левое значение';

comment on column public.operations.v2 is 'Правое значение';

comment on column public.operations.parentid is 'UUID родительской опреации';

comment on column public.operations."left" is 'Левый операнд родительского выражения?';

alter table public.operations
    owner to orchestrator;

