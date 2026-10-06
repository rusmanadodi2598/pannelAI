<script lang="ts">
	// The live drawing's edges (docs/DRAFT/012 F3; the beam is draft 015 F1, the split is draft 018, and the
	// request path is docs/DRAFT/043 F2).
	//
	// One SVG stretched over the drawing box, holding every hop of the path the request takes:
	// `Client >> Combo >> Gateway >> Upstream >> Response`. The hop itself (one line, or the beam that
	// travels it) is `UsageTopologyHop.svelte`, and this file decides only which hops exist and in which
	// direction each is drawn.
	//
	// The lines are drawn in the box's own units: `viewBox="0 0 100 100"` with `preserveAspectRatio="none"`
	// stretches the box, and a linear map sends a straight line to a straight line, so a line drawn between
	// two nodes' percentages ends exactly under those nodes. `non-scaling-stroke` keeps the stroke weight
	// from being stretched with it.
	//
	// The gateway-to-upstream hop is written from the gateway outward, as it has always been, because the
	// drawing's tests find a node's edge by the position of the line's far end: a hop is matched to the node
	// it lands on, not to the order the lists happen to be in.
	import UsageTopologyHop from './UsageTopologyHop.svelte';
	import {
		CLIENT_POSITION,
		GATEWAY_POSITION,
		RESPONSE_POSITION
	} from '$lib/schemas/usage-topology-geometry';
	import type { TopologyLayout } from '$lib/schemas/usage-topology-view';

	type Props = {
		/** Where every node and terminal is, which is where each hop starts and ends. */
		layout: TopologyLayout;
		/** Frames are arriving on the connection that is open now, so a beam may move. */
		live: boolean;
	};

	let { layout, live }: Props = $props();
</script>

<svg class="absolute inset-0 size-full" viewBox="0 0 100 100" preserveAspectRatio="none">
	<!-- A request that addressed no combo enters through this hop alone, so the gateway is never the only
	     thing on the path with a line that does not carry a name. It is lit by `direct`, not by the client
	     terminal: a combo request travels the combo's two hops, and a beam here would claim a request that
	     never took this one. -->
	<UsageTopologyHop
		from={CLIENT_POSITION}
		to={GATEWAY_POSITION}
		edge="client-gateway"
		id="client-gateway"
		state={layout.direct.state}
		{live}
	/>

	{#each layout.combos as combo (combo.key)}
		<UsageTopologyHop
			from={CLIENT_POSITION}
			to={combo}
			edge="client-combo"
			id={`client-combo:${combo.key}`}
			state={combo.state}
			{live}
		/>
		<UsageTopologyHop
			from={combo}
			to={GATEWAY_POSITION}
			edge="combo-gateway"
			id={`combo-gateway:${combo.key}`}
			state={combo.state}
			{live}
		/>
	{/each}

	{#each layout.providers as node (node.key)}
		<UsageTopologyHop
			from={GATEWAY_POSITION}
			to={node}
			edge="gateway-provider"
			id={`gateway-provider:${node.key}`}
			state={node.state}
			{live}
		/>
		<!-- The way back. An upstream that is routing is streaming its answer through here while the request
		     is still in flight, which is why this hop is lit by the provider's own state rather than by a
		     finished row the panel would have to time on its own. -->
		<UsageTopologyHop
			from={node}
			to={RESPONSE_POSITION}
			edge="provider-response"
			id={`provider-response:${node.key}`}
			state={node.state}
			{live}
		/>
	{/each}
</svg>
