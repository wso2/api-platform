import { Fade } from "@mui/material";

export interface AnimateFadeProps {
    children: React.ReactElement;
    show?: boolean;
    mountOnEnter?: boolean;
    unmountOnExit?: boolean;
}
export function AnimateFade(props: AnimateFadeProps) {
    const { children, show = true, mountOnEnter = true, unmountOnExit = true } = props;
    return (
        <Fade in={show} mountOnEnter={mountOnEnter} unmountOnExit={unmountOnExit}>
            {children}
        </Fade>
    )
}