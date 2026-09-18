<script lang="ts">
	// Test harness for PanelSidebar.
	//
	// The sidebar body needs the sidebar context and the tooltip provider that Sidebar.Provider installs,
	// so the smallest tree that renders it is Provider > PanelSidebar. Kept in tests/ so the assertion is
	// made against the real component rather than a copy of it.
	import PanelSidebar from '$lib/components/PanelSidebar.svelte';
	import * as Sidebar from '$lib/primitives/sidebar/index.js';
</script>

<Sidebar.Provider>
	<PanelSidebar />
</Sidebar.Provider>
