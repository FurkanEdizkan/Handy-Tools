<script lang="ts">
  /**
   * Bottom-center floating notification. `visible` is bindable; when
   * `duration` is set the toast auto-dismisses and flips it back to false.
   * Styled by the design system's `.toast` rules in styles/gui.css.
   */
  interface Props {
    message: string;
    tone?: 'info' | 'success' | 'error';
    visible?: boolean;
    /** Auto-dismiss delay in ms. 0 keeps the toast until closed. */
    duration?: number;
    onclose?: () => void;
  }

  let {
    message,
    tone = 'info',
    visible = $bindable(false),
    duration = 0,
    onclose,
  }: Props = $props();

  $effect(() => {
    if (!visible || duration <= 0) return;
    const handle = setTimeout(() => {
      visible = false;
      onclose?.();
    }, duration);
    return () => clearTimeout(handle);
  });
</script>

<div
  class="toast"
  class:show={visible}
  class:error={tone === 'error'}
  role="status"
  aria-live="polite"
>
  {message}
</div>
