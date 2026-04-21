[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)]
    [ValidateSet('Get', 'Set')]
    [string]$Action,

    [ValidateRange(0, 100)]
    [int]$Level = 0
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

$typeDefinition = @"
using System;
using System.Runtime.InteropServices;

namespace DeskCtrlAudio {
    enum EDataFlow {
        eRender,
        eCapture,
        eAll
    }

    enum ERole {
        eConsole,
        eMultimedia,
        eCommunications
    }

    [Flags]
    enum CLSCTX : uint {
        INPROC_SERVER = 0x1
    }

    [ComImport]
    [Guid("BCDE0395-E52F-467C-8E3D-C4579291692E")]
    class MMDeviceEnumeratorComObject {
    }

    [ComImport]
    [Guid("A95664D2-9614-4F35-A746-DE8DB63617E6")]
    [InterfaceType(ComInterfaceType.InterfaceIsIUnknown)]
    interface IMMDeviceEnumerator {
        int NotImpl1();
        int GetDefaultAudioEndpoint(EDataFlow dataFlow, ERole role, out IMMDevice endpoint);
    }

    [ComImport]
    [Guid("D666063F-1587-4E43-81F1-B948E807363F")]
    [InterfaceType(ComInterfaceType.InterfaceIsIUnknown)]
    interface IMMDevice {
        int Activate(ref Guid iid, CLSCTX clsctx, IntPtr activationParams, [MarshalAs(UnmanagedType.Interface)] out object interfacePointer);
    }

    [ComImport]
    [Guid("5CDF2C82-841E-4546-9722-0CF74078229A")]
    [InterfaceType(ComInterfaceType.InterfaceIsIUnknown)]
    interface IAudioEndpointVolume {
        int RegisterControlChangeNotify(IntPtr notify);
        int UnregisterControlChangeNotify(IntPtr notify);
        int GetChannelCount(out uint channelCount);
        int SetMasterVolumeLevel(float levelDb, Guid eventContext);
        int SetMasterVolumeLevelScalar(float level, Guid eventContext);
        int GetMasterVolumeLevel(out float levelDb);
        int GetMasterVolumeLevelScalar(out float level);
        int SetChannelVolumeLevel(uint channelNumber, float levelDb, Guid eventContext);
        int SetChannelVolumeLevelScalar(uint channelNumber, float level, Guid eventContext);
        int GetChannelVolumeLevel(uint channelNumber, out float levelDb);
        int GetChannelVolumeLevelScalar(uint channelNumber, out float level);
        int SetMute([MarshalAs(UnmanagedType.Bool)] bool isMuted, Guid eventContext);
        int GetMute(out bool isMuted);
        int GetVolumeStepInfo(out uint step, out uint stepCount);
        int VolumeStepUp(Guid eventContext);
        int VolumeStepDown(Guid eventContext);
        int QueryHardwareSupport(out uint hardwareSupportMask);
        int GetVolumeRange(out float volumeMindB, out float volumeMaxdB, out float volumeIncrementdB);
    }

    public static class AudioManager {
        static IAudioEndpointVolume GetEndpointVolume() {
            IMMDeviceEnumerator enumerator = (IMMDeviceEnumerator)(new MMDeviceEnumeratorComObject());
            IMMDevice device = null;
            try {
                Marshal.ThrowExceptionForHR(enumerator.GetDefaultAudioEndpoint(EDataFlow.eRender, ERole.eMultimedia, out device));
                Guid iid = typeof(IAudioEndpointVolume).GUID;
                object endpointObject;
                Marshal.ThrowExceptionForHR(device.Activate(ref iid, CLSCTX.INPROC_SERVER, IntPtr.Zero, out endpointObject));
                return (IAudioEndpointVolume)endpointObject;
            }
            finally {
                if (device != null) {
                    Marshal.ReleaseComObject(device);
                }
                Marshal.ReleaseComObject(enumerator);
            }
        }

        static int ToPercent(float scalar) {
            return (int)Math.Round(Math.Max(0.0f, Math.Min(1.0f, scalar)) * 100.0f, MidpointRounding.AwayFromZero);
        }

        public static int GetVolumePercent() {
            IAudioEndpointVolume endpoint = null;
            try {
                endpoint = GetEndpointVolume();
                float level;
                Marshal.ThrowExceptionForHR(endpoint.GetMasterVolumeLevelScalar(out level));
                return ToPercent(level);
            }
            finally {
                if (endpoint != null) {
                    Marshal.ReleaseComObject(endpoint);
                }
            }
        }

        public static int SetVolumePercent(int level) {
            IAudioEndpointVolume endpoint = null;
            try {
                endpoint = GetEndpointVolume();
                float scalar = Math.Max(0.0f, Math.Min(1.0f, level / 100.0f));
                Guid eventContext = Guid.Empty;
                Marshal.ThrowExceptionForHR(endpoint.SetMasterVolumeLevelScalar(scalar, eventContext));

                float updated;
                Marshal.ThrowExceptionForHR(endpoint.GetMasterVolumeLevelScalar(out updated));
                return ToPercent(updated);
            }
            finally {
                if (endpoint != null) {
                    Marshal.ReleaseComObject(endpoint);
                }
            }
        }
    }
}
"@

if (-not ('DeskCtrlAudio.AudioManager' -as [type])) {
    Add-Type -TypeDefinition $typeDefinition -Language CSharp
}

switch ($Action) {
    'Get' {
        [DeskCtrlAudio.AudioManager]::GetVolumePercent()
        break
    }
    'Set' {
        [DeskCtrlAudio.AudioManager]::SetVolumePercent($Level)
        break
    }
}
