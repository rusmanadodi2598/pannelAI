<script lang="ts">
	// Test harness for PanelHeader.
	//
	// The header renders the sidebar trigger, which reads the sidebar context, so the smallest tree that
	// renders it is Provider > PanelHeader. Kept in tests/ so the assertion is made against the real
	// component rather than a copy of it.
	import PanelHeader from '$lib/components/PanelHeader.svelte';
	import * as Sidebar from '$lib/primitives/sidebar/index.js';
</script>

<Sidebar.Provider>
	<PanelHeader />
</Sidebar.Provider>
