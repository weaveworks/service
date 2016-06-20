import React from 'react';
import { Motion, spring } from 'react-motion';

export class BackgroundContainer extends React.Component {

  constructor() {
    super();

    this.state = {
      shiftX: 50,
      shiftY: 50
    };

    this.handleMouseMove = this.handleMouseMove.bind(this);
  }

  handleMouseMove(ev) {
    const centerX = window.innerWidth / 2;
    const centerY = window.innerHeight / 2;
    const maxShiftPercent = 5;
    const shiftX = 50 + (centerX - ev.clientX) / centerX * maxShiftPercent;
    const shiftY = 50 + (centerY - ev.clientY) / centerY * maxShiftPercent;

    this.setState({ shiftX, shiftY });
  }

  render() {
    const motionConfig = [200, 20];
    const styles = {
      container: {
        height: '100%',
        overflowY: 'scroll'
      },
      motion: {
        shiftX: spring(this.state.shiftX, motionConfig),
        shiftY: spring(this.state.shiftY, motionConfig)
      }
    };
    const imageUrl = this.props.imageUrl;

    return (
      <div style={styles.container} onMouseMove={this.handleMouseMove}>
        <Motion style={styles.motion}>
          {({shiftX, shiftY}) =>
            <div style={{
              backgroundImage: `url("${imageUrl}")`,
              backgroundPosition: `${shiftX}% ${shiftY}%`,
              backgroundRepeat: 'no-repeat',
              position: 'absolute',
              top: 0,
              bottom: 0,
              left: 0,
              right: 0,
              zIndex: -1
            }}></div>
          }
        </Motion>
        {this.props.children}
      </div>
    );
  }
}
