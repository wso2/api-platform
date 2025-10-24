import { Slide } from "@mui/material";

export interface AnimateSlideProps {
    children: React.ReactElement;
    direction?: "up" | "down" | "left" | "right";
    show?: boolean;
    mountOnEnter?: boolean;
    unmountOnExit?: boolean;
}
export function AnimateSlide(props: AnimateSlideProps) {
    const { children, direction = "up", show = true, mountOnEnter = true, unmountOnExit = true } = props;
    return (
        <Slide direction={direction} in={show} mountOnEnter={mountOnEnter} unmountOnExit={unmountOnExit}>
            {children}
        </Slide>
    )
}