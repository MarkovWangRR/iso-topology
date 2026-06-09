# 03 — Grouping with containers

In step 02 we drew five flat services. Real architectures have
**boundaries** — a VPC, a cluster, a region. iso-topology has two
ways to express them:

1. `.d2` nested syntax — auto-translated to iso `group`
2. YAML `shape: group` — for precise placement

We'll do the `.d2` path first because it's the same source you wrote
in step 02 — just one extra brace.

## Wrap the API tier in a VPC

Rewrite `services.d2`:

```d2
user:  User { shape: person }
edge:  Edge LB

vpc: AWS VPC {
  api:    API Gateway
  worker: Worker
  db:     Database { shape: cylinder }
}

user    -> edge:        HTTPS
edge    -> vpc.api:     route
vpc.api -> vpc.worker:  enqueue
vpc.worker -> vpc.db:   write
```

Re-render:

```bash
isotopo render services.d2 ./out
open ./out/topology.html
```

The three services inside `vpc { … }` now sit on a translucent
labeled iso panel. Arrows from outside reference parts via
dot-notation: `vpc.api`, `vpc.worker`, `vpc.db`.

## Nesting goes deeper

You can nest any number of levels:

```d2
cluster: Kubernetes Cluster {
  region_a: Region A {
    pod: API Pod
    cache: Cache { shape: cylinder }
  }
  region_b: Region B {
    pod: API Pod
    cache: Cache { shape: cylinder }
  }
}
```

iso-topology emits one translucent substrate per level — outer
faded, inner stronger. Connectors reference full dot-paths:
`cluster.region_a.pod -> cluster.region_b.pod`.

## YAML equivalent (precise placement)

If you need pixel-precise control, the YAML composite DSL has the
same primitive — `shape: group`:

```yaml
nodes:
  scene:
    shape: composite
    parts:
      - id: vpc
        shape: group
        offset: {wx: 100, wy: 80}
        geom:   {w: 480, d: 280, h: 6}
        label:  "AWS VPC"
        parts:
          - id: api
            shape: rectangle
            offset: {wx: 40, wy: 40}
            geom:   {w: 120, d: 80, h: 30}
            label:  API
```

Children's `offset` is interpreted **relative to the group's
offset** — so `api` lands at world (140, 120) in the parent
composite.

Full grammar: [`reference/dsl-yaml.md`](../reference/dsl-yaml.md).

## Next step

→ [`04-replicas-annotations.md`](04-replicas-annotations.md) —
auto-replicate parts and add screen-space callouts.
