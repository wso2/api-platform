# **\<Feature Name\>**

<!--
The entry-point document. A reviewer should be able to read only this file and understand what
is being built, why, and what the shape of the solution is. Keep it decision-oriented — the
detailed mechanics belong in implementation.md.
-->

| | |
| :---- | :---- |
| Author(s) | \<name\> \<email\> |
| Status | Draft / Under review / Approved / Implemented |
| Last updated | \<YYYY-MM-DD\> |
| GitHub issue | \<link\> |
| Related docs | \<architecture review notes, discussions, proposals\> |

# What is the problem we are trying to solve?

<!--
State the current behaviour and where it falls short. Break the problem into named parts so
each one can be traced to a part of the solution below. Describe the gap, not the fix.
-->

\<One paragraph framing the current state and the gap.\>

The problem has \<n\> main parts:

* **\<Problem 1\>**

  \<What is missing today and what breaks because of it.\>

* **\<Problem 2\>**

  \<...\>

# Why should it be solved?

<!-- The user/operator-visible value. One bullet per benefit, each tied to a problem above. -->

* **\<Benefit\>** - \<who gets it and what it enables.\>
* **\<Benefit\>** - \<...\>

# Proposed Solution

<!--
The shape of the solution: what each component becomes responsible for, and the principles that
govern the design. Include the architecture diagram here.
-->

\<One or two paragraphs describing the approach end to end.\>

![][image1]

**\<Component A\> responsibilities**

* \<...\>
* \<...\>

**\<Component B\> responsibilities**

* \<...\>

**Design principles**

* **\<Principle\>** - \<the rule and why it holds.\>
* **\<Principle\>** - \<...\>

## **\<Sub-section: flow / payload / storage change\>**

<!--
Add one sub-section per significant part of the solution — a CLI flow, a changed payload shape,
a storage change. Keep the detail here at "what and why"; the "how" goes in implementation.md.
-->

\<Description.\>

```
<example payload / command sequence / config>
```

## **Key Behaviours**

<!-- The observable rules a reviewer or tester should be able to check against. -->

* **\<Behaviour\>** - \<what happens, under what condition.\>
* **\<Behaviour\>** - \<...\>

# Challenges and solutions

<!-- Every non-obvious problem the design had to work around, and how it is mitigated. -->

| Challenge | Solution |
| :---- | :---- |
| \<challenge\> | \<mitigation\> |
| \<challenge\> | \<mitigation\> |

# Out of Scope

<!-- What a reader might reasonably assume is included, but is not. State it explicitly. -->

* \<...\>

# Future Works

<!-- Deferred work, with enough context that a later phase can pick it up. -->

* \<...\>

<!-- [image1]: <path-or-data-uri> -->
