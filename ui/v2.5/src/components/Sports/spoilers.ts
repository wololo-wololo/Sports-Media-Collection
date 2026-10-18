import { useCallback } from "react";
import { useConfigureUISetting } from "src/core/StashService";
import { useConfigurationContext } from "src/hooks/Config";
import { useToast } from "src/hooks/Toast";

// The spoiler shield hides results (winners, pole sitters) until a
// recording of the session has been watched. It is on unless turned off.
export function useSpoilerShield() {
  const { configuration } = useConfigurationContext();
  const [saveUISetting] = useConfigureUISetting();
  const Toast = useToast();

  const enabled = configuration?.ui.sportsSpoilerShield !== false;

  const setEnabled = useCallback(
    async (value: boolean) => {
      try {
        await saveUISetting({
          variables: { key: "sportsSpoilerShield", value },
        });
      } catch (e) {
        Toast.error(e);
      }
    },
    [saveUISetting, Toast]
  );

  return { enabled, setEnabled };
}
